#include <iostream>
#include <memory>
#include <sstream>
#include <string>
#include <string.h>

#include <grpc/grpc.h>
#include <grpc++/channel.h>
#include <grpc++/client_context.h>
#include <grpc++/create_channel.h>
#include <grpc++/security/credentials.h>
#include "gobgp_api_client.grpc.pb.h"

extern "C" {
    // Gobgp library
    #include "libgobgp.h"
}

using grpc::Channel;
using grpc::ClientContext;
using grpc::Status;

using gobgpapi::GobgpApi;

class GrpcClient {
    public:
        GrpcClient(std::shared_ptr<Channel> channel) : stub_(GobgpApi::NewStub(channel)) {}

        std::string GetNeighbor() {
            gobgpapi::GetNeighborRequest request;

            ClientContext context;

            gobgpapi::GetNeighborResponse response;
            grpc::Status status = stub_->GetNeighbor(&context, request, &response);

            if (status.ok()) {
                std::stringstream buffer;
                for (int i=0; i < response.peers_size(); i++) {

                    gobgpapi::PeerConf peer_conf = response.peers(i).conf();
                    gobgpapi::PeerState peer_info = response.peers(i).info();
                    gobgpapi::Timers peer_timers = response.peers(i).timers();

                    buffer
                        << "BGP neighbor is: " << peer_conf.neighbor_address()
                        << ", remote AS: " << peer_conf.peer_as() << "\n"
                        << "\tBGP version: 4, remote route ID " << peer_conf.id() << "\n"
                        << "\tBGP state = " << peer_info.bgp_state()
                        << ", up for " << peer_timers.state().uptime() << "\n"
                        << "\tBGP OutQ = " << peer_info.out_q()
                        << ", Flops = " << peer_info.flops() << "\n"
                        << "\tHold time is " << peer_timers.state().hold_time()
                        << ", keepalive interval is " << peer_timers.state().keepalive_interval() << "seconds\n"
                        << "\tConfigured hold time is " << peer_timers.config().hold_time() << "\n";

                }
                return buffer.str();
            } else {
                std::stringstream buffer;
                buffer
                    << status.error_code() << "\n"
                    << status.error_message() << "\n"
                    << status.error_details() << "\n";
                return buffer.str();
            }
    }

    private:
        std::unique_ptr<GobgpApi::Stub> stub_;
};

int main(int argc, char** argv) {
    if(argc < 2) {
        std::cout << "Usage: ./gobgp_api_client [gobgp address]\n";
        return 1;
    }

    std::string addr = argv[1];
    GrpcClient gobgp_client(grpc::CreateChannel(addr + ":50051", grpc::InsecureChannelCredentials()));

    std::string reply = gobgp_client.GetNeighbor();
    std::cout << reply;
    
    return 0;
}
