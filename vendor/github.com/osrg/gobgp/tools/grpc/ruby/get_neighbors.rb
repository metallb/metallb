require 'gobgp_pb'
require 'gobgp_services_pb'

host = ARGV[0]

stub = Gobgpapi::GobgpApi::Stub.new("#{host}:50051", :this_channel_is_insecure)
arg = Gobgpapi::GetNeighborRequest.new()
stub.get_neighbor(arg).peers.each do |n|
    puts "BGP neighbor is #{n.conf.neighbor_address}, remote AS #{n.conf.peer_as}"
    puts "\tBGP version 4, remote route ID #{n.conf.id}"
    puts "\tBGP state = #{n.info.bgp_state}, up for #{n.timers.state.uptime}"
    puts "\tBGP OutQ = #{n.info.out_q}, Flops = #{n.info.flops}"
    puts "\tHold time is #{n.timers.state.hold_time}, keepalive interval is #{n.timers.state.keepalive_interval} seconds"
    puts "\tConfigured hold time is #{n.timers.config.hold_time}"
end
