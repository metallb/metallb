import os
from nose.plugins import Plugin

parser_option = None


class OptionParser(Plugin):

    def options(self, parser, env=os.environ):
        super(OptionParser, self).options(parser, env=env)
        parser.add_option('--test-prefix', action="store", dest="test_prefix", default="")
        parser.add_option('--gobgp-image', action="store", dest="gobgp_image", default="osrg/gobgp")
        parser.add_option('--exabgp-path', action="store", dest="exabgp_path", default="")
        parser.add_option('--go-path', action="store", dest="go_path", default="")
        parser.add_option('--gobgp-log-level', action="store",
                          dest="gobgp_log_level", default="info")
        parser.add_option('--test-index', action="store", type="int", dest="test_index", default=0)
        parser.add_option('--config-format', action="store", dest="config_format", default="yaml")

    def configure(self, options, conf):
        super(OptionParser, self).configure(options, conf)
        global parser_option
        parser_option = options

        if not self.enabled:
            return

    def finalize(self, result):
        pass
