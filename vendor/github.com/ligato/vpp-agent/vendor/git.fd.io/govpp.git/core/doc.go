// Package core provides connectivity to VPP via the adapter: sends and receives the messages to/from VPP,
// marshalls/unmarshalls them and forwards them between the client Go channels and the VPP.
//
// The interface_plugin APIs the core exposes is tied to a connection: Connect provides a connection, that cane be
// later used to request an API channel via NewAPIChannel / NewAPIChannelBuffered functions:
//
//	conn, err := govpp.Connect()
//	if err != nil {
//		// handle error!
//	}
//	defer conn.Disconnect()
//
//	ch, err := conn.NewAPIChannel()
//	if err != nil {
//		// handle error!
//	}
//	defer ch.Close()
//
// Note that one application can open only one connection, that can serve multiple API channels.
//
// The API offers two ways of communication with govpp core: using Go channels, or using convenient function
// wrappers over the Go channels. The latter should be sufficient for most of the use cases.
//
// The entry point to the API is the Channel structure, that can be obtained from the existing connection using
// the NewAPIChannel or NewAPIChannelBuffered functions:
//
//	conn, err := govpp.Connect()
//	if err != nil {
//		// handle error!
//	}
//	defer conn.Disconnect()
//
//	ch, err := conn.NewAPIChannel()
//	if err != nil {
//		// handle error!
//	}
//	defer ch.Close()
//
//
// Simple Request-Reply API
//
// The simple version of the API is based on blocking SendRequest / ReceiveReply calls, where a single request
// message is sent  to VPP and a single reply message is filled in when the reply comes from VPP:
//
// 	req := &acl.ACLPluginGetVersion{}
// 	reply := &acl.ACLPluginGetVersionReply{}
//
// 	err := ch.SendRequest(req).ReceiveReply(reply)
// 	// process the reply
//
// Note that if the reply message type that comes from VPP does not match with provided one, you'll get an error.
//
//
// Multipart Reply API
//
// If multiple messages are expected as a reply to a request, SendMultiRequest API must be used:
//
// 	req := &interfaces.SwInterfaceDump{}
// 	reqCtx := ch.SendMultiRequest(req)
//
//	for {
// 		reply := &interfaces.SwInterfaceDetails{}
// 		stop, err := reqCtx.ReceiveReply(reply)
// 		if stop {
// 			break // break out of the loop
//		}
//		// process the reply
// 	}
//
// Note that if the last reply has been already consumed, stop boolean return value is set to true.
// Do not use the message itself if stop is true - it won't be filled with actual data.
//
//
// Go Channels API
//
// The blocking API introduced above may be not sufficient for some management applications that strongly
// rely on usage of Go channels. In this case, the API allows to access the underlying Go channels directly, e.g.
// the following replacement of the SendRequest / ReceiveReply API:
//
// 	req := &acl.ACLPluginGetVersion{}
// 	// send the request to the request go channel
// 	ch.GetRequestChannel <- &api.VppRequest{Message: req}
//
// 	// receive a reply from the reply go channel
// 	vppReply := <-ch.GetReplyChannel
//
// 	// decode the message
// 	reply := &acl.ACLPluginGetVersionReply{}
// 	err := ch.MsgDecoder.DecodeMsg(vppReply.Data, reply)
//
// 	// process the reply
//
//
// Notifications API
//
// to subscribe for receiving of the specified notification messages via provided Go channel, use the
// SubscribeNotification API:
//
// 	// subscribe for specific notification message
//	notifChan := make(chan api.Message, 100)
//	subs, _ := ch.SubscribeNotification(notifChan, interfaces.NewSwInterfaceSetFlags)
//
//	// receive one notification
//	notif := (<-notifChan).(*interfaces.SwInterfaceSetFlags)
//
//	ch.UnsubscribeNotification(subs)
//
// Note that the caller is responsible for creating the Go channel with preferred buffer size. If the channel's
// buffer is full, the notifications will not be delivered into it.
//
package core
