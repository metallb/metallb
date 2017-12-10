var grpc = require('grpc');
var api = grpc.load('gobgp.proto').gobgpapi;
var stub = new api.GobgpApi('localhost:50051', grpc.credentials.createInsecure());

stub.getNeighbor({}, function(err, neighbor) {
  neighbor.peers.forEach(function(peer) {
    if(peer.info.bgp_state == 'BGP_FSM_ESTABLISHED') {
      var date = new Date(Number(peer.timers.state.uptime)*1000);
      var holdtime = peer.timers.state.negotiated_hold_time;
      var keepalive = peer.timers.state.keepalive_interval;
    }

    console.log('BGP neighbor:', peer.conf.neighbor_address,
                ', remote AS:', peer.conf.peer_as);
    console.log("\tBGP version 4, remote router ID:", peer.conf.id);
    console.log("\tBGP state:", peer.info.bgp_state,
                ', uptime:', date);
    console.log("\tBGP OutQ:", peer.info.out_q,
                ', Flops:', peer.info.flops);
    console.log("\tHold time:", holdtime,
                ', keepalive interval:', keepalive, 'seconds');
    console.log("\tConfigured hold time:", peer.timers.config.hold_time);
  });
});
