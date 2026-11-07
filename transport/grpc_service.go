package transport

import (
	"context"

	"google.golang.org/grpc"
)

type p2pTransportServer interface {
	Gossip(grpc.BidiStreamingServer[grpcStreamMessage, grpcStreamMessage]) error
}

type p2pTransportClient interface {
	Gossip(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[grpcStreamMessage, grpcStreamMessage], error)
}

type p2pTransportClientImpl struct{ connection grpc.ClientConnInterface }

func newP2PTransportClient(connection grpc.ClientConnInterface) p2pTransportClient {
	return &p2pTransportClientImpl{connection: connection}
}

func (client *p2pTransportClientImpl) Gossip(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[grpcStreamMessage, grpcStreamMessage], error) {
	stream, err := client.connection.NewStream(ctx, &p2pTransportServiceDesc.Streams[0], "/vexo.transport.P2P/Gossip", opts...)
	if err != nil {
		return nil, err
	}
	return &grpc.GenericClientStream[grpcStreamMessage, grpcStreamMessage]{ClientStream: stream}, nil
}

func registerP2PTransportServer(server *grpc.Server, service p2pTransportServer) {
	server.RegisterService(&p2pTransportServiceDesc, service)
}

var p2pTransportServiceDesc = grpc.ServiceDesc{
	ServiceName: "vexo.transport.P2P",
	HandlerType: (*p2pTransportServer)(nil),
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Gossip",
			Handler:       p2pTransportGossipHandler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
}

func p2pTransportGossipHandler(service any, stream grpc.ServerStream) error {
	return service.(p2pTransportServer).Gossip(&p2pTransportGossipServer{ServerStream: stream})
}

type p2pTransportGossipServer struct{ grpc.ServerStream }

func (server *p2pTransportGossipServer) Send(message *grpcStreamMessage) error {
	return server.ServerStream.SendMsg(message)
}

func (server *p2pTransportGossipServer) Recv() (*grpcStreamMessage, error) {
	message := new(grpcStreamMessage)
	if err := server.ServerStream.RecvMsg(message); err != nil {
		return nil, err
	}
	return message, nil
}
