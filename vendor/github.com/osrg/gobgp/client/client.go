// Copyright (C) 2016 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package client provides a wrapper for GoBGP's gRPC API
package client

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"
)

type Client struct {
	conn *grpc.ClientConn
	cli  api.GobgpApiClient
}

func defaultGRPCOptions() []grpc.DialOption {
	return []grpc.DialOption{grpc.WithTimeout(time.Second), grpc.WithBlock(), grpc.WithInsecure()}
}

// New returns a new Client using the given target and options for dialing
// to the grpc server. If an error occurs during dialing it will be returned and
// Client will be nil.
func New(target string, opts ...grpc.DialOption) (*Client, error) {
	return NewWith(context.Background(), target, opts...)
}

// NewWith is like New, but uses the given ctx to cancel or expire the current
// attempt to connect if it becomes Done before the connection succeeds.
func NewWith(ctx context.Context, target string, opts ...grpc.DialOption) (*Client, error) {
	if target == "" {
		target = ":50051"
	}
	if len(opts) == 0 {
		opts = defaultGRPCOptions()
	}
	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		return nil, err
	}
	cli := api.NewGobgpApiClient(conn)
	return &Client{conn: conn, cli: cli}, nil
}

// NewFrom returns a new Client, using the given conn and cli for the
// underlying connection. The given grpc.ClientConn connection is expected to be
// initialized and paired with the api client. See New to have the connection
// dialed for you.
func NewFrom(conn *grpc.ClientConn, cli api.GobgpApiClient) *Client {
	return &Client{conn: conn, cli: cli}
}

func (cli *Client) Close() error {
	return cli.conn.Close()
}

func (cli *Client) StartServer(c *config.Global) error {
	_, err := cli.cli.StartServer(context.Background(), &api.StartServerRequest{
		Global: &api.Global{
			As:               c.Config.As,
			RouterId:         c.Config.RouterId,
			ListenPort:       c.Config.Port,
			ListenAddresses:  c.Config.LocalAddressList,
			UseMultiplePaths: c.UseMultiplePaths.Config.Enabled,
		},
	})
	return err
}

func (cli *Client) StopServer() error {
	_, err := cli.cli.StopServer(context.Background(), &api.StopServerRequest{})
	return err
}

func (cli *Client) GetServer() (*config.Global, error) {
	ret, err := cli.cli.GetServer(context.Background(), &api.GetServerRequest{})
	if err != nil {
		return nil, err
	}
	return &config.Global{
		Config: config.GlobalConfig{
			As:               ret.Global.As,
			RouterId:         ret.Global.RouterId,
			Port:             ret.Global.ListenPort,
			LocalAddressList: ret.Global.ListenAddresses,
		},
		UseMultiplePaths: config.UseMultiplePaths{
			Config: config.UseMultiplePathsConfig{
				Enabled: ret.Global.UseMultiplePaths,
			},
		},
	}, nil
}

func (cli *Client) EnableZebra(c *config.Zebra) error {
	req := &api.EnableZebraRequest{
		Url:                  c.Config.Url,
		Version:              uint32(c.Config.Version),
		NexthopTriggerEnable: c.Config.NexthopTriggerEnable,
		NexthopTriggerDelay:  uint32(c.Config.NexthopTriggerDelay),
	}
	for _, t := range c.Config.RedistributeRouteTypeList {
		req.RouteTypes = append(req.RouteTypes, string(t))
	}
	_, err := cli.cli.EnableZebra(context.Background(), req)
	return err
}

func (cli *Client) getNeighbor(name string, afi int, vrf string, enableAdvertised bool) ([]*config.Neighbor, error) {
	ret, err := cli.cli.GetNeighbor(context.Background(), &api.GetNeighborRequest{EnableAdvertised: enableAdvertised, Address: name})
	if err != nil {
		return nil, err
	}

	neighbors := make([]*config.Neighbor, 0, len(ret.Peers))

	for _, p := range ret.Peers {
		if name != "" && name != p.Info.NeighborAddress && name != p.Conf.NeighborInterface {
			continue
		}
		if vrf != "" && name != p.Conf.Vrf {
			continue
		}
		if afi > 0 {
			v6 := net.ParseIP(p.Info.NeighborAddress).To4() == nil
			if afi == bgp.AFI_IP && v6 || afi == bgp.AFI_IP6 && !v6 {
				continue
			}
		}
		n, err := api.NewNeighborFromAPIStruct(p)
		if err != nil {
			return nil, err
		}
		neighbors = append(neighbors, n)
	}
	return neighbors, nil
}

func (cli *Client) ListNeighbor() ([]*config.Neighbor, error) {
	return cli.getNeighbor("", 0, "", false)
}

func (cli *Client) ListNeighborByTransport(afi int) ([]*config.Neighbor, error) {
	return cli.getNeighbor("", afi, "", false)
}

func (cli *Client) ListNeighborByVRF(vrf string) ([]*config.Neighbor, error) {
	return cli.getNeighbor("", 0, vrf, false)
}

func (cli *Client) GetNeighbor(name string, options ...bool) (*config.Neighbor, error) {
	enableAdvertised := false
	if len(options) > 0 && options[0] {
		enableAdvertised = true
	}
	ns, err := cli.getNeighbor(name, 0, "", enableAdvertised)
	if err != nil {
		return nil, err
	}
	if len(ns) == 0 {
		return nil, fmt.Errorf("not found neighbor %s", name)
	}
	return ns[0], nil
}

func (cli *Client) AddNeighbor(c *config.Neighbor) error {
	peer := api.NewPeerFromConfigStruct(c)
	_, err := cli.cli.AddNeighbor(context.Background(), &api.AddNeighborRequest{Peer: peer})
	return err
}

func (cli *Client) DeleteNeighbor(c *config.Neighbor) error {
	peer := api.NewPeerFromConfigStruct(c)
	_, err := cli.cli.DeleteNeighbor(context.Background(), &api.DeleteNeighborRequest{Peer: peer})
	return err
}

func (cli *Client) UpdateNeighbor(c *config.Neighbor, doSoftResetIn bool) (bool, error) {
	peer := api.NewPeerFromConfigStruct(c)
	response, err := cli.cli.UpdateNeighbor(context.Background(), &api.UpdateNeighborRequest{Peer: peer, DoSoftResetIn: doSoftResetIn})
	return response.NeedsSoftResetIn, err
}

func (cli *Client) ShutdownNeighbor(addr, communication string) error {
	_, err := cli.cli.ShutdownNeighbor(context.Background(), &api.ShutdownNeighborRequest{Address: addr, Communication: communication})
	return err
}

func (cli *Client) ResetNeighbor(addr, communication string) error {
	_, err := cli.cli.ResetNeighbor(context.Background(), &api.ResetNeighborRequest{Address: addr, Communication: communication})
	return err
}

func (cli *Client) EnableNeighbor(addr string) error {
	_, err := cli.cli.EnableNeighbor(context.Background(), &api.EnableNeighborRequest{Address: addr})
	return err
}

func (cli *Client) DisableNeighbor(addr, communication string) error {
	_, err := cli.cli.DisableNeighbor(context.Background(), &api.DisableNeighborRequest{Address: addr, Communication: communication})
	return err
}

func (cli *Client) softreset(addr string, family bgp.RouteFamily, dir api.SoftResetNeighborRequest_SoftResetDirection) error {
	_, err := cli.cli.SoftResetNeighbor(context.Background(), &api.SoftResetNeighborRequest{
		Address:   addr,
		Direction: dir,
	})
	return err
}

func (cli *Client) SoftResetIn(addr string, family bgp.RouteFamily) error {
	return cli.softreset(addr, family, api.SoftResetNeighborRequest_IN)
}

func (cli *Client) SoftResetOut(addr string, family bgp.RouteFamily) error {
	return cli.softreset(addr, family, api.SoftResetNeighborRequest_OUT)
}

func (cli *Client) SoftReset(addr string, family bgp.RouteFamily) error {
	return cli.softreset(addr, family, api.SoftResetNeighborRequest_BOTH)
}

func (cli *Client) getRIB(resource api.Resource, name string, family bgp.RouteFamily, prefixes []*table.LookupPrefix) (*table.Table, error) {
	prefixList := make([]*api.TableLookupPrefix, 0, len(prefixes))
	for _, p := range prefixes {
		prefixList = append(prefixList, &api.TableLookupPrefix{
			Prefix:       p.Prefix,
			LookupOption: api.TableLookupOption(p.LookupOption),
		})
	}
	stream, err := cli.cli.GetPath(context.Background(), &api.GetPathRequest{
		Type:     resource,
		Family:   uint32(family),
		Name:     name,
		Prefixes: prefixList,
	})
	if err != nil {
		return nil, err
	}
	pathMap := make(map[string][]*table.Path)
	for {
		p, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		nlri, err := p.GetNativeNlri()
		if err != nil {
			return nil, err
		}
		var path *table.Path
		if p.Identifier > 0 {
			path, err = p.ToNativePath()
		} else {
			path, err = p.ToNativePath(api.ToNativeOption{
				NLRI: nlri,
			})
		}
		if err != nil {
			return nil, err
		}
		nlriStr := nlri.String()
		pathMap[nlriStr] = append(pathMap[nlriStr], path)
	}
	dstList := make([]*table.Destination, 0, len(pathMap))
	for _, pathList := range pathMap {
		dstList = append(dstList, table.NewDestination(pathList[0].GetNlri(), 0, pathList...))
	}
	return table.NewTable(family, dstList...), nil
}

func (cli *Client) GetRIB(family bgp.RouteFamily, prefixes []*table.LookupPrefix) (*table.Table, error) {
	return cli.getRIB(api.Resource_GLOBAL, "", family, prefixes)
}

func (cli *Client) GetLocalRIB(name string, family bgp.RouteFamily, prefixes []*table.LookupPrefix) (*table.Table, error) {
	return cli.getRIB(api.Resource_LOCAL, name, family, prefixes)
}

func (cli *Client) GetAdjRIBIn(name string, family bgp.RouteFamily, prefixes []*table.LookupPrefix) (*table.Table, error) {
	return cli.getRIB(api.Resource_ADJ_IN, name, family, prefixes)
}

func (cli *Client) GetAdjRIBOut(name string, family bgp.RouteFamily, prefixes []*table.LookupPrefix) (*table.Table, error) {
	return cli.getRIB(api.Resource_ADJ_OUT, name, family, prefixes)
}

func (cli *Client) GetVRFRIB(name string, family bgp.RouteFamily, prefixes []*table.LookupPrefix) (*table.Table, error) {
	return cli.getRIB(api.Resource_VRF, name, family, prefixes)
}

func (cli *Client) getRIBInfo(resource api.Resource, name string, family bgp.RouteFamily) (*table.TableInfo, error) {
	res, err := cli.cli.GetRibInfo(context.Background(), &api.GetRibInfoRequest{
		Info: &api.TableInfo{
			Type:   resource,
			Name:   name,
			Family: uint32(family),
		},
	})
	if err != nil {
		return nil, err
	}
	return &table.TableInfo{
		NumDestination: int(res.Info.NumDestination),
		NumPath:        int(res.Info.NumPath),
		NumAccepted:    int(res.Info.NumAccepted),
	}, nil

}

func (cli *Client) GetRIBInfo(family bgp.RouteFamily) (*table.TableInfo, error) {
	return cli.getRIBInfo(api.Resource_GLOBAL, "", family)
}

func (cli *Client) GetLocalRIBInfo(name string, family bgp.RouteFamily) (*table.TableInfo, error) {
	return cli.getRIBInfo(api.Resource_LOCAL, name, family)
}

func (cli *Client) GetAdjRIBInInfo(name string, family bgp.RouteFamily) (*table.TableInfo, error) {
	return cli.getRIBInfo(api.Resource_ADJ_IN, name, family)
}

func (cli *Client) GetAdjRIBOutInfo(name string, family bgp.RouteFamily) (*table.TableInfo, error) {
	return cli.getRIBInfo(api.Resource_ADJ_OUT, name, family)
}

type AddPathByStreamClient struct {
	stream api.GobgpApi_InjectMrtClient
}

func (c *AddPathByStreamClient) Send(paths ...*table.Path) error {
	ps := make([]*api.Path, 0, len(paths))
	for _, p := range paths {
		ps = append(ps, api.ToPathApiInBin(p, nil))
	}
	return c.stream.Send(&api.InjectMrtRequest{
		Resource: api.Resource_GLOBAL,
		Paths:    ps,
	})
}

func (c *AddPathByStreamClient) Close() error {
	_, err := c.stream.CloseAndRecv()
	return err
}

func (cli *Client) AddPathByStream() (*AddPathByStreamClient, error) {
	stream, err := cli.cli.InjectMrt(context.Background())
	if err != nil {
		return nil, err
	}
	return &AddPathByStreamClient{stream}, nil
}

func (cli *Client) addPath(vrfID string, pathList []*table.Path) ([]byte, error) {
	resource := api.Resource_GLOBAL
	if vrfID != "" {
		resource = api.Resource_VRF
	}
	var uuid []byte
	for _, path := range pathList {
		r, err := cli.cli.AddPath(context.Background(), &api.AddPathRequest{
			Resource: resource,
			VrfId:    vrfID,
			Path:     api.ToPathApi(path, nil),
		})
		if err != nil {
			return nil, err
		}
		uuid = r.Uuid
	}
	return uuid, nil
}

func (cli *Client) AddPath(pathList []*table.Path) ([]byte, error) {
	return cli.addPath("", pathList)
}

func (cli *Client) AddVRFPath(vrfID string, pathList []*table.Path) ([]byte, error) {
	if vrfID == "" {
		return nil, fmt.Errorf("VRF ID is empty")
	}
	return cli.addPath(vrfID, pathList)
}

func (cli *Client) deletePath(uuid []byte, f bgp.RouteFamily, vrfID string, pathList []*table.Path) error {
	var reqs []*api.DeletePathRequest

	resource := api.Resource_GLOBAL
	if vrfID != "" {
		resource = api.Resource_VRF
	}
	switch {
	case len(pathList) != 0:
		for _, path := range pathList {
			reqs = append(reqs, &api.DeletePathRequest{
				Resource: resource,
				VrfId:    vrfID,
				Path:     api.ToPathApi(path, nil),
			})
		}
	default:
		reqs = append(reqs, &api.DeletePathRequest{
			Resource: resource,
			VrfId:    vrfID,
			Uuid:     uuid,
			Family:   uint32(f),
		})
	}

	for _, req := range reqs {
		if _, err := cli.cli.DeletePath(context.Background(), req); err != nil {
			return err
		}
	}
	return nil
}

func (cli *Client) DeletePath(pathList []*table.Path) error {
	return cli.deletePath(nil, bgp.RouteFamily(0), "", pathList)
}

func (cli *Client) DeleteVRFPath(vrfID string, pathList []*table.Path) error {
	if vrfID == "" {
		return fmt.Errorf("VRF ID is empty")
	}
	return cli.deletePath(nil, bgp.RouteFamily(0), vrfID, pathList)
}

func (cli *Client) DeletePathByUUID(uuid []byte) error {
	return cli.deletePath(uuid, bgp.RouteFamily(0), "", nil)
}

func (cli *Client) DeletePathByFamily(family bgp.RouteFamily) error {
	return cli.deletePath(nil, family, "", nil)
}

func (cli *Client) GetVRF() ([]*api.Vrf, error) {
	ret, err := cli.cli.GetVrf(context.Background(), &api.GetVrfRequest{})
	if err != nil {
		return nil, err
	}
	return ret.Vrfs, nil
}

func (cli *Client) AddVRF(name string, id int, rd bgp.RouteDistinguisherInterface, im, ex []bgp.ExtendedCommunityInterface) error {
	arg := &api.AddVrfRequest{
		Vrf: &api.Vrf{
			Name:     name,
			Rd:       api.MarshalRD(rd),
			Id:       uint32(id),
			ImportRt: api.MarshalRTs(im),
			ExportRt: api.MarshalRTs(ex),
		},
	}
	_, err := cli.cli.AddVrf(context.Background(), arg)
	return err
}

func (cli *Client) DeleteVRF(name string) error {
	arg := &api.DeleteVrfRequest{
		Vrf: &api.Vrf{
			Name: name,
		},
	}
	_, err := cli.cli.DeleteVrf(context.Background(), arg)
	return err
}

func (cli *Client) getDefinedSet(typ table.DefinedType, name string) ([]table.DefinedSet, error) {
	ret, err := cli.cli.GetDefinedSet(context.Background(), &api.GetDefinedSetRequest{
		Type: api.DefinedType(typ),
		Name: name,
	})
	if err != nil {
		return nil, err
	}
	ds := make([]table.DefinedSet, 0, len(ret.Sets))
	for _, s := range ret.Sets {
		d, err := api.NewDefinedSetFromApiStruct(s)
		if err != nil {
			return nil, err
		}
		ds = append(ds, d)
	}
	return ds, nil
}

func (cli *Client) GetDefinedSet(typ table.DefinedType) ([]table.DefinedSet, error) {
	return cli.getDefinedSet(typ, "")
}

func (cli *Client) GetDefinedSetByName(typ table.DefinedType, name string) (table.DefinedSet, error) {
	sets, err := cli.getDefinedSet(typ, name)
	if err != nil {
		return nil, err
	}
	if len(sets) == 0 {
		return nil, fmt.Errorf("not found defined set: %s", name)
	} else if len(sets) > 1 {
		return nil, fmt.Errorf("invalid response for GetDefinedSetByName")
	}
	return sets[0], nil
}

func (cli *Client) AddDefinedSet(d table.DefinedSet) error {
	a, err := api.NewAPIDefinedSetFromTableStruct(d)
	if err != nil {
		return err
	}
	_, err = cli.cli.AddDefinedSet(context.Background(), &api.AddDefinedSetRequest{
		Set: a,
	})
	return err
}

func (cli *Client) DeleteDefinedSet(d table.DefinedSet, all bool) error {
	a, err := api.NewAPIDefinedSetFromTableStruct(d)
	if err != nil {
		return err
	}
	_, err = cli.cli.DeleteDefinedSet(context.Background(), &api.DeleteDefinedSetRequest{
		Set: a,
		All: all,
	})
	return err
}

func (cli *Client) ReplaceDefinedSet(d table.DefinedSet) error {
	a, err := api.NewAPIDefinedSetFromTableStruct(d)
	if err != nil {
		return err
	}
	_, err = cli.cli.ReplaceDefinedSet(context.Background(), &api.ReplaceDefinedSetRequest{
		Set: a,
	})
	return err
}

func (cli *Client) GetStatement() ([]*table.Statement, error) {
	ret, err := cli.cli.GetStatement(context.Background(), &api.GetStatementRequest{})
	if err != nil {
		return nil, err
	}
	sts := make([]*table.Statement, 0, len(ret.Statements))
	for _, s := range ret.Statements {
		st, err := api.NewStatementFromApiStruct(s)
		if err != nil {
			return nil, err
		}
		sts = append(sts, st)
	}
	return sts, nil
}

func (cli *Client) AddStatement(t *table.Statement) error {
	a := api.NewAPIStatementFromTableStruct(t)
	_, err := cli.cli.AddStatement(context.Background(), &api.AddStatementRequest{
		Statement: a,
	})
	return err
}

func (cli *Client) DeleteStatement(t *table.Statement, all bool) error {
	a := api.NewAPIStatementFromTableStruct(t)
	_, err := cli.cli.DeleteStatement(context.Background(), &api.DeleteStatementRequest{
		Statement: a,
		All:       all,
	})
	return err
}

func (cli *Client) ReplaceStatement(t *table.Statement) error {
	a := api.NewAPIStatementFromTableStruct(t)
	_, err := cli.cli.ReplaceStatement(context.Background(), &api.ReplaceStatementRequest{
		Statement: a,
	})
	return err
}

func (cli *Client) GetPolicy() ([]*table.Policy, error) {
	ret, err := cli.cli.GetPolicy(context.Background(), &api.GetPolicyRequest{})
	if err != nil {
		return nil, err
	}
	pols := make([]*table.Policy, 0, len(ret.Policies))
	for _, p := range ret.Policies {
		pol, err := api.NewPolicyFromApiStruct(p)
		if err != nil {
			return nil, err
		}
		pols = append(pols, pol)
	}
	return pols, nil
}

func (cli *Client) AddPolicy(t *table.Policy, refer bool) error {
	a := api.NewAPIPolicyFromTableStruct(t)
	_, err := cli.cli.AddPolicy(context.Background(), &api.AddPolicyRequest{
		Policy:                  a,
		ReferExistingStatements: refer,
	})
	return err
}

func (cli *Client) DeletePolicy(t *table.Policy, all, preserve bool) error {
	a := api.NewAPIPolicyFromTableStruct(t)
	_, err := cli.cli.DeletePolicy(context.Background(), &api.DeletePolicyRequest{
		Policy:             a,
		All:                all,
		PreserveStatements: preserve,
	})
	return err
}

func (cli *Client) ReplacePolicy(t *table.Policy, refer, preserve bool) error {
	a := api.NewAPIPolicyFromTableStruct(t)
	_, err := cli.cli.ReplacePolicy(context.Background(), &api.ReplacePolicyRequest{
		Policy:                  a,
		ReferExistingStatements: refer,
		PreserveStatements:      preserve,
	})
	return err
}

func (cli *Client) getPolicyAssignment(name string, dir table.PolicyDirection) (*table.PolicyAssignment, error) {
	var typ api.PolicyType
	switch dir {
	case table.POLICY_DIRECTION_IMPORT:
		typ = api.PolicyType_IMPORT
	case table.POLICY_DIRECTION_EXPORT:
		typ = api.PolicyType_EXPORT
	}
	resource := api.Resource_GLOBAL
	if name != "" {
		resource = api.Resource_LOCAL
	}

	ret, err := cli.cli.GetPolicyAssignment(context.Background(), &api.GetPolicyAssignmentRequest{
		Assignment: &api.PolicyAssignment{
			Name:     name,
			Resource: resource,
			Type:     typ,
		},
	})
	if err != nil {
		return nil, err
	}

	def := table.ROUTE_TYPE_ACCEPT
	if ret.Assignment.Default == api.RouteAction_REJECT {
		def = table.ROUTE_TYPE_REJECT
	}

	pols := make([]*table.Policy, 0, len(ret.Assignment.Policies))
	for _, p := range ret.Assignment.Policies {
		pol, err := api.NewPolicyFromApiStruct(p)
		if err != nil {
			return nil, err
		}
		pols = append(pols, pol)
	}
	return &table.PolicyAssignment{
		Name:     name,
		Type:     dir,
		Policies: pols,
		Default:  def,
	}, nil
}

func (cli *Client) GetImportPolicy() (*table.PolicyAssignment, error) {
	return cli.getPolicyAssignment("", table.POLICY_DIRECTION_IMPORT)
}

func (cli *Client) GetExportPolicy() (*table.PolicyAssignment, error) {
	return cli.getPolicyAssignment("", table.POLICY_DIRECTION_EXPORT)
}

func (cli *Client) GetRouteServerImportPolicy(name string) (*table.PolicyAssignment, error) {
	return cli.getPolicyAssignment(name, table.POLICY_DIRECTION_IMPORT)
}

func (cli *Client) GetRouteServerExportPolicy(name string) (*table.PolicyAssignment, error) {
	return cli.getPolicyAssignment(name, table.POLICY_DIRECTION_EXPORT)
}

func (cli *Client) AddPolicyAssignment(assignment *table.PolicyAssignment) error {
	_, err := cli.cli.AddPolicyAssignment(context.Background(), &api.AddPolicyAssignmentRequest{
		Assignment: api.NewAPIPolicyAssignmentFromTableStruct(assignment),
	})
	return err
}

func (cli *Client) DeletePolicyAssignment(assignment *table.PolicyAssignment, all bool) error {
	a := api.NewAPIPolicyAssignmentFromTableStruct(assignment)
	_, err := cli.cli.DeletePolicyAssignment(context.Background(), &api.DeletePolicyAssignmentRequest{
		Assignment: a,
		All:        all})
	return err
}

func (cli *Client) ReplacePolicyAssignment(assignment *table.PolicyAssignment) error {
	_, err := cli.cli.ReplacePolicyAssignment(context.Background(), &api.ReplacePolicyAssignmentRequest{
		Assignment: api.NewAPIPolicyAssignmentFromTableStruct(assignment),
	})
	return err
}

//func (cli *Client) EnableMrt(c *config.MrtConfig) error {
//}
//
//func (cli *Client) DisableMrt(c *config.MrtConfig) error {
//}
//

func (cli *Client) GetRPKI() ([]*config.RpkiServer, error) {
	rsp, err := cli.cli.GetRpki(context.Background(), &api.GetRpkiRequest{})
	if err != nil {
		return nil, err
	}
	servers := make([]*config.RpkiServer, 0, len(rsp.Servers))
	for _, s := range rsp.Servers {
		// Note: RpkiServerConfig.Port is uint32 type, but the TCP/UDP port is
		// 16-bit length.
		port, err := strconv.ParseUint(s.Conf.RemotePort, 10, 16)
		if err != nil {
			return nil, err
		}
		server := &config.RpkiServer{
			Config: config.RpkiServerConfig{
				Address: s.Conf.Address,
				Port:    uint32(port),
			},
			State: config.RpkiServerState{
				Up:           s.State.Up,
				SerialNumber: s.State.Serial,
				RecordsV4:    s.State.RecordIpv4,
				RecordsV6:    s.State.RecordIpv6,
				PrefixesV4:   s.State.PrefixIpv4,
				PrefixesV6:   s.State.PrefixIpv6,
				Uptime:       s.State.Uptime,
				Downtime:     s.State.Downtime,
				RpkiMessages: config.RpkiMessages{
					RpkiReceived: config.RpkiReceived{
						SerialNotify:  s.State.SerialNotify,
						CacheReset:    s.State.CacheReset,
						CacheResponse: s.State.CacheResponse,
						Ipv4Prefix:    s.State.ReceivedIpv4,
						Ipv6Prefix:    s.State.ReceivedIpv6,
						EndOfData:     s.State.EndOfData,
						Error:         s.State.Error,
					},
					RpkiSent: config.RpkiSent{
						SerialQuery: s.State.SerialQuery,
						ResetQuery:  s.State.ResetQuery,
					},
				},
			},
		}
		servers = append(servers, server)
	}
	return servers, nil
}

func (cli *Client) GetROA(family bgp.RouteFamily) ([]*table.ROA, error) {
	rsp, err := cli.cli.GetRoa(context.Background(), &api.GetRoaRequest{
		Family: uint32(family),
	})
	if err != nil {
		return nil, err
	}
	return api.NewROAListFromApiStructList(rsp.Roas)
}

func (cli *Client) AddRPKIServer(address string, port, lifetime int) error {
	_, err := cli.cli.AddRpki(context.Background(), &api.AddRpkiRequest{
		Address:  address,
		Port:     uint32(port),
		Lifetime: int64(lifetime),
	})
	return err
}

func (cli *Client) DeleteRPKIServer(address string) error {
	_, err := cli.cli.DeleteRpki(context.Background(), &api.DeleteRpkiRequest{
		Address: address,
	})
	return err
}

func (cli *Client) EnableRPKIServer(address string) error {
	_, err := cli.cli.EnableRpki(context.Background(), &api.EnableRpkiRequest{
		Address: address,
	})
	return err
}

func (cli *Client) DisableRPKIServer(address string) error {
	_, err := cli.cli.DisableRpki(context.Background(), &api.DisableRpkiRequest{
		Address: address,
	})
	return err
}

func (cli *Client) ResetRPKIServer(address string) error {
	_, err := cli.cli.ResetRpki(context.Background(), &api.ResetRpkiRequest{
		Address: address,
	})
	return err
}

func (cli *Client) SoftResetRPKIServer(address string) error {
	_, err := cli.cli.SoftResetRpki(context.Background(), &api.SoftResetRpkiRequest{
		Address: address,
	})
	return err
}

func (cli *Client) ValidateRIBWithRPKI(prefixes ...string) error {
	req := &api.ValidateRibRequest{}
	if len(prefixes) > 1 {
		return fmt.Errorf("too many prefixes: %d", len(prefixes))
	} else if len(prefixes) == 1 {
		req.Prefix = prefixes[0]
	}
	_, err := cli.cli.ValidateRib(context.Background(), req)
	return err
}

func (cli *Client) AddBMP(c *config.BmpServerConfig) error {
	_, err := cli.cli.AddBmp(context.Background(), &api.AddBmpRequest{
		Address: c.Address,
		Port:    c.Port,
		Type:    api.AddBmpRequest_MonitoringPolicy(c.RouteMonitoringPolicy.ToInt()),
	})
	return err
}

func (cli *Client) DeleteBMP(c *config.BmpServerConfig) error {
	_, err := cli.cli.DeleteBmp(context.Background(), &api.DeleteBmpRequest{
		Address: c.Address,
		Port:    c.Port,
	})
	return err
}

type MonitorRIBClient struct {
	stream api.GobgpApi_MonitorRibClient
}

func (c *MonitorRIBClient) Recv() (*table.Destination, error) {
	d, err := c.stream.Recv()
	if err != nil {
		return nil, err
	}
	return d.ToNativeDestination()
}

func (cli *Client) MonitorRIB(family bgp.RouteFamily, current bool) (*MonitorRIBClient, error) {
	stream, err := cli.cli.MonitorRib(context.Background(), &api.MonitorRibRequest{
		Table: &api.Table{
			Type:   api.Resource_GLOBAL,
			Family: uint32(family),
		},
		Current: current,
	})
	if err != nil {
		return nil, err
	}
	return &MonitorRIBClient{stream}, nil
}

func (cli *Client) MonitorAdjRIBIn(name string, family bgp.RouteFamily, current bool) (*MonitorRIBClient, error) {
	stream, err := cli.cli.MonitorRib(context.Background(), &api.MonitorRibRequest{
		Table: &api.Table{
			Type:   api.Resource_ADJ_IN,
			Name:   name,
			Family: uint32(family),
		},
		Current: current,
	})
	if err != nil {
		return nil, err
	}
	return &MonitorRIBClient{stream}, nil
}

type MonitorNeighborStateClient struct {
	stream api.GobgpApi_MonitorPeerStateClient
}

func (c *MonitorNeighborStateClient) Recv() (*config.Neighbor, error) {
	p, err := c.stream.Recv()
	if err != nil {
		return nil, err
	}
	return api.NewNeighborFromAPIStruct(p)
}

func (cli *Client) MonitorNeighborState(name string, current bool) (*MonitorNeighborStateClient, error) {
	stream, err := cli.cli.MonitorPeerState(context.Background(), &api.Arguments{
		Name:    name,
		Current: current,
	})
	if err != nil {
		return nil, err
	}
	return &MonitorNeighborStateClient{stream}, nil
}
