package consul

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"net/rpc"
	"os"
	"sort"
	"testing"
	"time"
)

func TestCatalogRegister(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"master"},
			Port:    8000,
		},
	}
	var out struct{}

	err := client.Call("Catalog.Register", &arg, &out)
	if err == nil || err.Error() != "No cluster leader" {
		t.Fatalf("err: %v", err)
	}

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	if err := client.Call("Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalogRegister_ForwardLeader(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client1 := rpcClient(t, s1)
	defer client1.Close()

	dir2, s2 := testServer(t)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	client2 := rpcClient(t, s2)
	defer client2.Close()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for a leader
	time.Sleep(100 * time.Millisecond)

	// Use the follower as the client
	var client *rpc.Client
	if !s1.IsLeader() {
		client = client1
	} else {
		client = client2
	}

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"master"},
			Port:    8000,
		},
	}
	var out struct{}
	if err := client.Call("Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalogRegister_ForwardDC(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfWANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinWAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for the leaders
	time.Sleep(100 * time.Millisecond)

	arg := structs.RegisterRequest{
		Datacenter: "dc2", // SHould forward through s1
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"master"},
			Port:    8000,
		},
	}
	var out struct{}
	if err := client.Call("Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalogDeregister(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	arg := structs.DeregisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
	}
	var out struct{}

	err := client.Call("Catalog.Deregister", &arg, &out)
	if err == nil || err.Error() != "No cluster leader" {
		t.Fatalf("err: %v", err)
	}

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	if err := client.Call("Catalog.Deregister", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalogListDatacenters(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfWANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinWAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	var out []string
	if err := client.Call("Catalog.ListDatacenters", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Sort the dcs
	sort.Strings(out)

	if len(out) != 2 {
		t.Fatalf("bad: %v", out)
	}
	if out[0] != "dc1" {
		t.Fatalf("bad: %v", out)
	}
	if out[1] != "dc2" {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalogListNodes(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var out structs.IndexedNodes
	err := client.Call("Catalog.ListNodes", &args, &out)
	if err == nil || err.Error() != "No cluster leader" {
		t.Fatalf("err: %v", err)
	}

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	// Just add a node
	s1.fsm.State().EnsureNode(1, structs.Node{"foo", "127.0.0.1"})

	if err := client.Call("Catalog.ListNodes", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(out.Nodes) != 2 {
		t.Fatalf("bad: %v", out)
	}

	// Server node is auto added from Serf
	if out.Nodes[0].Node != s1.config.NodeName {
		t.Fatalf("bad: %v", out)
	}
	if out.Nodes[1].Node != "foo" {
		t.Fatalf("bad: %v", out)
	}
	if out.Nodes[1].Address != "127.0.0.1" {
		t.Fatalf("bad: %v", out)
	}
}

func BenchmarkCatalogListNodes(t *testing.B) {
	dir1, s1 := testServer(nil)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(nil, s1)
	defer client.Close()

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	// Just add a node
	s1.fsm.State().EnsureNode(1, structs.Node{"foo", "127.0.0.1"})

	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	for i := 0; i < t.N; i++ {
		var out structs.IndexedNodes
		if err := client.Call("Catalog.ListNodes", &args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
}

func TestCatalogListServices(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var out structs.IndexedServices
	err := client.Call("Catalog.ListServices", &args, &out)
	if err == nil || err.Error() != "No cluster leader" {
		t.Fatalf("err: %v", err)
	}

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	// Just add a node
	s1.fsm.State().EnsureNode(1, structs.Node{"foo", "127.0.0.1"})
	s1.fsm.State().EnsureService(2, "foo", &structs.NodeService{"db", "db", []string{"primary"}, 5000})

	if err := client.Call("Catalog.ListServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(out.Services) != 2 {
		t.Fatalf("bad: %v", out)
	}
	// Consul service should auto-register
	if _, ok := out.Services["consul"]; !ok {
		t.Fatalf("bad: %v", out)
	}
	if len(out.Services["db"]) != 1 {
		t.Fatalf("bad: %v", out)
	}
	if out.Services["db"][0] != "primary" {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalogListServices_Blocking(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var out structs.IndexedServices

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	// Run the query
	if err := client.Call("Catalog.ListServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Setup a blocking query
	args.MinQueryIndex = out.Index
	args.MaxQueryTime = time.Second

	// Async cause a change
	start := time.Now()
	go func() {
		time.Sleep(100 * time.Millisecond)
		s1.fsm.State().EnsureNode(1, structs.Node{"foo", "127.0.0.1"})
		s1.fsm.State().EnsureService(2, "foo", &structs.NodeService{"db", "db", []string{"primary"}, 5000})
	}()

	// Re-run the query
	out = structs.IndexedServices{}
	if err := client.Call("Catalog.ListServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should block at least 100ms
	if time.Now().Sub(start) < 100*time.Millisecond {
		t.Fatalf("too fast")
	}

	// Check the indexes
	if out.Index != 2 {
		t.Fatalf("bad: %v", out)
	}

	// Should find the service
	if len(out.Services) != 2 {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalogListServices_Timeout(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var out structs.IndexedServices

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	// Run the query
	if err := client.Call("Catalog.ListServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Setup a blocking query
	args.MinQueryIndex = out.Index
	args.MaxQueryTime = 100 * time.Millisecond

	// Re-run the query
	start := time.Now()
	out = structs.IndexedServices{}
	if err := client.Call("Catalog.ListServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should block at least 100ms
	if time.Now().Sub(start) < 100*time.Millisecond {
		t.Fatalf("too fast")
	}

	// Check the indexes, should not change
	if out.Index != args.MinQueryIndex {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalogListServiceNodes(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	args := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		ServiceTag:  "slave",
		TagFilter:   false,
	}
	var out structs.IndexedServiceNodes
	err := client.Call("Catalog.ServiceNodes", &args, &out)
	if err == nil || err.Error() != "No cluster leader" {
		t.Fatalf("err: %v", err)
	}

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	// Just add a node
	s1.fsm.State().EnsureNode(1, structs.Node{"foo", "127.0.0.1"})
	s1.fsm.State().EnsureService(2, "foo", &structs.NodeService{"db", "db", []string{"primary"}, 5000})

	if err := client.Call("Catalog.ServiceNodes", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(out.ServiceNodes) != 1 {
		t.Fatalf("bad: %v", out)
	}

	// Try with a filter
	args.TagFilter = true
	out = structs.IndexedServiceNodes{}

	if err := client.Call("Catalog.ServiceNodes", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out.ServiceNodes) != 0 {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalogNodeServices(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	args := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       "foo",
	}
	var out structs.IndexedNodeServices
	err := client.Call("Catalog.NodeServices", &args, &out)
	if err == nil || err.Error() != "No cluster leader" {
		t.Fatalf("err: %v", err)
	}

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	// Just add a node
	s1.fsm.State().EnsureNode(1, structs.Node{"foo", "127.0.0.1"})
	s1.fsm.State().EnsureService(2, "foo", &structs.NodeService{"db", "db", []string{"primary"}, 5000})
	s1.fsm.State().EnsureService(3, "foo", &structs.NodeService{"web", "web", nil, 80})

	if err := client.Call("Catalog.NodeServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if out.NodeServices.Node.Address != "127.0.0.1" {
		t.Fatalf("bad: %v", out)
	}
	if len(out.NodeServices.Services) != 2 {
		t.Fatalf("bad: %v", out)
	}
	services := out.NodeServices.Services
	if !strContains(services["db"].Tags, "primary") || services["db"].Port != 5000 {
		t.Fatalf("bad: %v", out)
	}
	if services["web"].Tags != nil || services["web"].Port != 80 {
		t.Fatalf("bad: %v", out)
	}
}

// Used to check for a regression against a known bug
func TestCatalogRegister_FailedCase1(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.2",
		Service: &structs.NodeService{
			Service: "web",
			Tags:    nil,
			Port:    8000,
		},
	}
	var out struct{}

	err := client.Call("Catalog.Register", &arg, &out)
	if err == nil || err.Error() != "No cluster leader" {
		t.Fatalf("err: %v", err)
	}

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	if err := client.Call("Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check we can get this back
	query := &structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "web",
	}
	var out2 structs.IndexedServiceNodes
	if err := client.Call("Catalog.ServiceNodes", query, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the output
	if len(out2.ServiceNodes) != 1 {
		t.Fatalf("Bad: %v", out2)
	}
}
