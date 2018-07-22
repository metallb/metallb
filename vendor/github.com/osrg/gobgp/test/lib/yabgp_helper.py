#!/usr/bin/env python

from __future__ import absolute_import
from __future__ import print_function

import json
import logging
import sys

from flask import Blueprint
import flask

from yabgp.agent import prepare_service
from yabgp.common import constants
from yabgp.handler import BaseHandler
from yabgp.api import app
from yabgp.api import utils as api_utils
from yabgp.api.v1 import auth

LOG = logging.getLogger(__name__)
blueprint = Blueprint('v1-ext', __name__)
_send_update = api_utils.send_update


def _extract_afi_safi(attr):
    # MP_REACH_NLRI(14) or MP_UNREACH_NLRI(15)
    nlri = attr.get(14, None) or attr.get(15, None)
    if nlri and 'afi_safi' in nlri:
        afi_safi = nlri.get('afi_safi', None)
        return constants.AFI_SAFI_DICT.get(tuple(afi_safi), None)
    return 'ipv4'


def _construct_msg(attr, nlri, withdraw):
    return {
        'afi_safi': _extract_afi_safi(attr),
        'attr': attr,
        'nlir': nlri or [],
        'withdraw': withdraw or [],
    }


def _extract_nlri_list(msg):
    try:
        if msg['afi_safi'] == 'ipv4':
            return msg['nlri']
        return msg['attr'][14]['nlri']  # MP_REACH_NLRI(14)
    except KeyError:
        # Ignore case when no nlri
        pass
    return []


def _extract_withdraw_list(msg):
    try:
        if msg['afi_safi'] == 'ipv4':
            return msg['withdraw']
        return msg['attr'][15]['withdraw']  # MP_UNREACH_NLRI(15)
    except KeyError:
        # Ignore case when no withdraw
        pass
    return []


def _nlri_key(nlri):
    if isinstance(nlri, (dict, list)):
        return json.dumps(nlri)
    return str(nlri)


# ADJ_RIB_IN = {
#     <peer.factory.peer_addr>: {
#         <afi_safi>: {
#             <nlri>: <msg>,
#             ...
#         },
#         ...
#     },
#     ...
# }
ADJ_RIB_IN = {}


@blueprint.route('/peer/<peer_ip>/adj-rib-in')
@auth.login_required
@api_utils.log_request
def peer_adj_rib_in(peer_ip):
    """
    Dumps one peer's adj-RIB-in.
    """
    return flask.jsonify(ADJ_RIB_IN.get(peer_ip, {}))


# ADJ_RIB_OUT = {
#     <peer_ip>: {
#         <afi_safi>: {
#             <nlri>: <msg>,
#             ...
#         },
#         ...
#     },
#     ...
# }
ADJ_RIB_OUT = {}


@blueprint.route('/peer/<peer_ip>/adj-rib-out')
@auth.login_required
@api_utils.log_request
def peer_adj_rib_out(peer_ip):
    """
    Dumps one peer's adj-RIB-out.
    """
    return flask.jsonify(ADJ_RIB_OUT.get(peer_ip, {}))


app.app.register_blueprint(blueprint, url_prefix='/v1-ext')


def send_update(peer_ip, attr, nlri, withdraw):
    """
    Wrapper of "yabgp.api.send_update" in order to hook sending UPDATE
    messages via REST API.
    """
    msg = _construct_msg(attr, nlri, withdraw)
    afi_safi = msg['afi_safi']
    ADJ_RIB_OUT.setdefault(peer_ip, {})
    ADJ_RIB_OUT[peer_ip].setdefault(afi_safi, {})
    rib = ADJ_RIB_OUT[peer_ip][afi_safi]
    for _nlri in _extract_nlri_list(msg):
        rib[_nlri_key(_nlri)] = msg
    for _withdraw in _extract_withdraw_list(msg):
        rib.pop(_nlri_key(_withdraw), None)
    return _send_update(peer_ip, attr, nlri, withdraw)


setattr(api_utils, 'send_update', send_update)


class CliHandler(BaseHandler):

    def __init__(self):
        super(CliHandler, self).__init__()

    def init(self):
        pass

    def on_update_error(self, peer, timestamp, msg):
        LOG.info('[-] UPDATE ERROR: %s', msg)

    def route_refresh_received(self, peer, msg, msg_type):
        LOG.info('[+] ROUTE_REFRESH received: %s', msg)

    def keepalive_received(self, peer, timestamp):
        LOG.debug('[+] KEEPALIVE received: %s', peer.factory.peer_addr)

    def open_received(self, peer, timestamp, result):
        LOG.info('[+] OPEN received: %s', result)

    def update_received(self, peer, timestamp, msg):
        LOG.info('[+] UPDATE received: %s', msg)
        peer_addr = peer.factory.peer_addr
        afi_safi = msg['afi_safi']
        ADJ_RIB_IN.setdefault(peer.factory.peer_addr, {})
        ADJ_RIB_IN[peer_addr].setdefault(afi_safi, {})
        rib = ADJ_RIB_IN[peer_addr][afi_safi]
        for nlri in _extract_nlri_list(msg):
            rib[_nlri_key(nlri)] = msg
        for withdraw in _extract_withdraw_list(msg):
            rib.pop(_nlri_key(withdraw), None)

    def notification_received(self, peer, msg):
        LOG.info('[-] NOTIFICATION received: %s', msg)

    def on_connection_lost(self, peer):
        LOG.info('[-] CONNECTION lost: %s', peer.factory.peer_addr)

    def on_connection_failed(self, peer, msg):
        LOG.info('[-] CONNECTION failed: %s', msg)


def main():
    try:
        cli_handler = CliHandler()
        prepare_service(handler=cli_handler)
    except Exception as e:
        print(e)


if __name__ == '__main__':
    sys.exit(main())
