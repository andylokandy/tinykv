// Copyright 2016 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"time"

	"github.com/pingcap-incubator/tinykv/proto/pkg/metapb"
	"github.com/pingcap-incubator/tinykv/proto/pkg/pdpb"
	"github.com/pingcap-incubator/tinykv/scheduler/pkg/testutil"
	"github.com/pingcap-incubator/tinykv/scheduler/server/core"
	. "github.com/pingcap/check"
)

var _ = Suite(&testClusterWorkerSuite{})

type testClusterWorkerSuite struct {
	baseCluster
}

func (s *testClusterWorkerSuite) TestReportSplit(c *C) {
	var cluster RaftCluster
	left := &metapb.Region{Id: 1, StartKey: []byte("a"), EndKey: []byte("b")}
	right := &metapb.Region{Id: 2, StartKey: []byte("b"), EndKey: []byte("c")}
	_, err := cluster.handleReportSplit(&pdpb.ReportSplitRequest{Left: left, Right: right})
	c.Assert(err, IsNil)
	_, err = cluster.handleReportSplit(&pdpb.ReportSplitRequest{Left: right, Right: left})
	c.Assert(err, NotNil)
}

func (s *testClusterWorkerSuite) TestReportBatchSplit(c *C) {
	var cluster RaftCluster
	regions := []*metapb.Region{
		{Id: 1, StartKey: []byte(""), EndKey: []byte("a")},
		{Id: 2, StartKey: []byte("a"), EndKey: []byte("b")},
		{Id: 3, StartKey: []byte("b"), EndKey: []byte("c")},
		{Id: 3, StartKey: []byte("c"), EndKey: []byte("")},
	}
	_, err := cluster.handleBatchReportSplit(&pdpb.ReportBatchSplitRequest{Regions: regions})
	c.Assert(err, IsNil)
}

func (s *testClusterWorkerSuite) TestValidRequestRegion(c *C) {
	var err error
	var cleanup func()
	s.svr, cleanup, err = NewTestServer(c)
	defer cleanup()
	c.Assert(err, IsNil)
	mustWaitLeader(c, []*Server{s.svr})
	s.grpcPDClient = testutil.MustNewGrpcClient(c, s.svr.GetAddr())
	_, err = s.svr.bootstrapCluster(s.newBootstrapRequest(c, s.svr.clusterID, "127.0.0.1:0"))
	c.Assert(err, IsNil)

	cluster := s.svr.GetRaftCluster()
	c.Assert(cluster, NotNil)

	r1 := core.NewRegionInfo(&metapb.Region{
		Id:       1,
		StartKey: []byte(""),
		EndKey:   []byte("a"),
		Peers: []*metapb.Peer{{
			Id:      1,
			StoreId: 1,
		}},
		RegionEpoch: &metapb.RegionEpoch{ConfVer: 2, Version: 2},
	}, &metapb.Peer{
		Id:      1,
		StoreId: 1,
	})
	err = cluster.HandleRegionHeartbeat(r1)
	c.Assert(err, IsNil)
	r2 := &metapb.Region{Id: 2, StartKey: []byte("a"), EndKey: []byte("b")}
	c.Assert(cluster.validRequestRegion(r2), NotNil)
	r3 := &metapb.Region{Id: 1, StartKey: []byte(""), EndKey: []byte("a"), RegionEpoch: &metapb.RegionEpoch{ConfVer: 1, Version: 2}}
	c.Assert(cluster.validRequestRegion(r3), NotNil)
	r4 := &metapb.Region{Id: 1, StartKey: []byte(""), EndKey: []byte("a"), RegionEpoch: &metapb.RegionEpoch{ConfVer: 2, Version: 1}}
	c.Assert(cluster.validRequestRegion(r4), NotNil)
	r5 := &metapb.Region{Id: 1, StartKey: []byte(""), EndKey: []byte("a"), RegionEpoch: &metapb.RegionEpoch{ConfVer: 2, Version: 2}}
	c.Assert(cluster.validRequestRegion(r5), IsNil)
	cluster.stop()
}

func (s *testClusterWorkerSuite) TestAskSplit(c *C) {
	var err error
	var cleanup func()
	s.svr, cleanup, err = NewTestServer(c)
	defer cleanup()
	c.Assert(err, IsNil)
	mustWaitLeader(c, []*Server{s.svr})
	s.grpcPDClient = testutil.MustNewGrpcClient(c, s.svr.GetAddr())
	_, err = s.svr.bootstrapCluster(s.newBootstrapRequest(c, s.svr.clusterID, "127.0.0.1:0"))
	c.Assert(err, IsNil)
	s.svr.SetClusterVersion("3.1.0")

	cluster := s.svr.GetRaftCluster()
	c.Assert(cluster, NotNil)
	cluster.opt.SetSplitMergeInterval(time.Hour)
	regions := cluster.GetRegions()

	req := &pdpb.AskSplitRequest{
		Header: &pdpb.RequestHeader{
			ClusterId: s.svr.ClusterID(),
		},
		Region: regions[0].GetMeta(),
	}

	_, err = cluster.handleAskSplit(req)
	c.Assert(err, IsNil)

	req1 := &pdpb.AskBatchSplitRequest{
		Header: &pdpb.RequestHeader{
			ClusterId: s.svr.ClusterID(),
		},
		Region:     regions[0].GetMeta(),
		SplitCount: 10,
	}

	_, err = cluster.handleAskBatchSplit(req1)
	c.Assert(err, IsNil)
}
