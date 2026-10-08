// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	g := New()
	assert.NotNil(t, g)
	assert.Equal(t, 0, g.GetNodeSize())
	assert.Equal(t, "Graph is empty.", g.String())
}

func TestInit(t *testing.T) {
	g := New()
	assert.True(t, g.AddNode(NewNode("a")))
	assert.Equal(t, 1, g.GetNodeSize())
	// Init 以 *d = *New() 重置图，清空所有节点
	g.Init()
	assert.Equal(t, 0, g.GetNodeSize())
}

func TestNewNode(t *testing.T) {
	n := NewNode("a")
	assert.Equal(t, "a", n.ID)
	assert.Equal(t, "white", n.Color)
	assert.NotNil(t, n.WeightTo)
	assert.NotNil(t, n.WeightFrom)
	assert.Equal(t, 0, len(n.WeightTo))
	assert.Equal(t, 0, len(n.WeightFrom))
}

func TestAddNode(t *testing.T) {
	g := New()
	// nil 节点被拒绝
	assert.False(t, g.AddNode(nil))
	assert.Equal(t, 0, g.GetNodeSize())

	// 首次添加成功
	assert.True(t, g.AddNode(NewNode("a")))
	assert.Equal(t, 1, g.GetNodeSize())

	// 重复 ID 被拒绝
	assert.False(t, g.AddNode(NewNode("a")))
	assert.Equal(t, 1, g.GetNodeSize())

	// 同一指针重复添加被拒绝
	nd := NewNode("b")
	assert.True(t, g.AddNode(nd))
	assert.False(t, g.AddNode(nd))
	assert.Equal(t, 2, g.GetNodeSize())
}

func TestGetNodeByID(t *testing.T) {
	g := New()
	g.AddNode(NewNode("a"))
	g.AddNode(NewNode("b"))

	assert.Equal(t, "a", g.GetNodeByID("a").ID)
	assert.Equal(t, "b", g.GetNodeByID("b").ID)
	assert.Nil(t, g.GetNodeByID("missing"))
}

func TestConnectAndGetEdgeWeight(t *testing.T) {
	g := New()
	a, b, c := NewNode("a"), NewNode("b"), NewNode("c")

	// nil 端点不产生边、不添加节点
	g.Connect(nil, b, 1)
	assert.Equal(t, 0, g.GetNodeSize())

	g.Connect(a, b, 1.5)
	g.Connect(b, c, 2.5)

	assert.Equal(t, 3, g.GetNodeSize())
	assert.Equal(t, float32(1.5), g.GetEdgeWeight(a, b))
	assert.Equal(t, float32(2.5), g.GetEdgeWeight(b, c))
	// 不存在的边权重为 0
	assert.Equal(t, float32(0), g.GetEdgeWeight(a, c))
	// nil 端点权重为 0
	assert.Equal(t, float32(0), g.GetEdgeWeight(nil, b))
}

func TestConnectDuplicateNodeReusesExisting(t *testing.T) {
	g := New()
	a, b := NewNode("a"), NewNode("b")
	g.Connect(a, b, 1)

	// 用重复 ID 的新指针连接，应复用已有节点、更新边权重，而非新增节点
	aDup := NewNode("a")
	g.Connect(aDup, b, 9)
	assert.Equal(t, 2, g.GetNodeSize())
	assert.Equal(t, float32(9), g.GetEdgeWeight(a, b))
}

func TestGetEdges(t *testing.T) {
	g := New()
	a, b, c := NewNode("a"), NewNode("b"), NewNode("c")
	g.Connect(a, b, 1.5)
	g.Connect(b, c, 2.5)

	edges := g.GetEdges()
	assert.Equal(t, 2, len(edges))

	weights := map[string]float32{}
	for _, e := range edges {
		weights[e.Src.ID+"->"+e.Dst.ID] = e.Weight
	}
	assert.Equal(t, float32(1.5), weights["a->b"])
	assert.Equal(t, float32(2.5), weights["b->c"])
	_, ok := weights["a->c"]
	assert.False(t, ok)
}

func TestUpdateEdgeWeight(t *testing.T) {
	g := New()
	a, b := NewNode("a"), NewNode("b")
	g.Connect(a, b, 1)
	assert.Equal(t, float32(1), g.GetEdgeWeight(a, b))

	g.UpdateEdgeWeight(a, b, 7)
	assert.Equal(t, float32(7), g.GetEdgeWeight(a, b))

	// nil 端点为空操作
	g.UpdateEdgeWeight(nil, b, 3)
	assert.Equal(t, float32(7), g.GetEdgeWeight(a, b))
}

func TestDeleteEdge(t *testing.T) {
	g := New()
	a, b, c := NewNode("a"), NewNode("b"), NewNode("c")
	g.Connect(a, b, 1)
	g.Connect(b, c, 2)

	g.DeleteEdge(a, b)
	assert.Equal(t, float32(0), g.GetEdgeWeight(a, b))
	// 其它边不受影响
	assert.Equal(t, float32(2), g.GetEdgeWeight(b, c))
	// 节点不随边删除
	assert.Equal(t, 3, g.GetNodeSize())

	// nil 端点为空操作
	g.DeleteEdge(nil, b)
	assert.Equal(t, float32(2), g.GetEdgeWeight(b, c))
}

func TestDeleteNode(t *testing.T) {
	g := New()
	a, b, c := NewNode("a"), NewNode("b"), NewNode("c")
	g.Connect(a, b, 1)
	g.Connect(b, c, 2)
	g.Connect(c, a, 3)

	g.DeleteNode(b)
	assert.Equal(t, 2, g.GetNodeSize())
	assert.Nil(t, g.GetNodeByID("b"))
	// 其它节点指向 b 的边被清除（a.WeightTo[b] 被 DeleteNode 移除）
	assert.Equal(t, float32(0), g.GetEdgeWeight(a, b))
	// 与被删节点无关的边保留
	assert.Equal(t, float32(3), g.GetEdgeWeight(c, a))
	// 图中不再有任何涉及 b 的边
	for _, e := range g.GetEdges() {
		assert.NotEqual(t, "b", e.Src.ID)
		assert.NotEqual(t, "b", e.Dst.ID)
	}

	// nil 为空操作
	g.DeleteNode(nil)
	assert.Equal(t, 2, g.GetNodeSize())
}

func TestTopologicalDag(t *testing.T) {
	// a -> b -> c，应为 DAG，结果中 a 早于 b，b 早于 c
	g := New()
	a, b, c := NewNode("a"), NewNode("b"), NewNode("c")
	g.Connect(a, b, 1)
	g.Connect(b, c, 1)

	result, ok := g.TopologicalDag()
	assert.True(t, ok)
	assert.Equal(t, 3, len(result))

	indexOf := func(id string) int {
		for i, nd := range result {
			if nd.ID == id {
				return i
			}
		}
		return -1
	}
	ia, ib, ic := indexOf("a"), indexOf("b"), indexOf("c")
	assert.NotEqual(t, -1, ia)
	assert.NotEqual(t, -1, ib)
	assert.NotEqual(t, -1, ic)
	assert.True(t, ia < ib)
	assert.True(t, ib < ic)
}

func TestTopologicalDag_Empty(t *testing.T) {
	g := New()
	result, ok := g.TopologicalDag()
	assert.True(t, ok)
	assert.Equal(t, 0, len(result))
}

func TestTopologicalDag_Cycle(t *testing.T) {
	// a <-> b 形成环，应返回 false 且结果为 nil
	g := New()
	a, b := NewNode("a"), NewNode("b")
	g.Connect(a, b, 1)
	g.Connect(b, a, 1)

	result, ok := g.TopologicalDag()
	assert.False(t, ok)
	assert.Nil(t, result)
}

func TestClone(t *testing.T) {
	// 连通图 a -> b -> c（Clone 仅从首个可达分量深拷贝，须用连通图）
	g := New()
	a, b, c := NewNode("a"), NewNode("b"), NewNode("c")
	g.Connect(a, b, 1.5)
	g.Connect(b, c, 2.5)

	cloned := g.Clone()
	assert.NotNil(t, cloned)
	assert.Equal(t, 3, cloned.GetNodeSize())

	// 克隆图保留权重与结构
	ca, cb := cloned.GetNodeByID("a"), cloned.GetNodeByID("b")
	cc := cloned.GetNodeByID("c")
	assert.Equal(t, float32(1.5), cloned.GetEdgeWeight(ca, cb))
	assert.Equal(t, float32(2.5), cloned.GetEdgeWeight(cb, cc))

	// 克隆出的节点是全新对象，与原图节点相互独立
	assert.True(t, a != ca)

	// 克隆图自身仍是 DAG，可被拓扑排序
	result, ok := cloned.TopologicalDag()
	assert.True(t, ok)
	assert.Equal(t, 3, len(result))
}

func TestClone_Empty(t *testing.T) {
	g := New()
	cloned := g.Clone()
	assert.NotNil(t, cloned)
	assert.Equal(t, 0, cloned.GetNodeSize())
}

func TestNodesGet(t *testing.T) {
	ns := Nodes{NewNode("x"), NewNode("y")}
	assert.Equal(t, "x", ns.Get("x").ID)
	assert.Equal(t, "y", ns.Get("y").ID)
	assert.Nil(t, ns.Get("z"))
}

func TestNodeString(t *testing.T) {
	n := NewNode("a")
	s := n.String()
	assert.Contains(t, s, "a")
	assert.Contains(t, s, "0 Outgoing")
	assert.Contains(t, s, "0 Incoming")

	g := New()
	a, b := NewNode("a"), NewNode("b")
	g.Connect(a, b, 1)
	s = a.String()
	assert.Contains(t, s, "1 Outgoing")
}

func TestDataString_NonEmpty(t *testing.T) {
	g := New()
	a, b := NewNode("a"), NewNode("b")
	g.Connect(a, b, 1)
	s := g.String()
	assert.Contains(t, s, "Graph has 2 Nodes")
	assert.Contains(t, s, "a")
	assert.Contains(t, s, "b")
}
