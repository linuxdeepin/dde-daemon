// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package graph

// Edge connects from Src to Dst with weight.
type Edge struct {
	Src    *Node
	Dst    *Node
	Weight float32
}

// GetEdges returns all edges of a graph.
func (d *Data) GetEdges() []Edge {
	rs := []Edge{}
	for nd1 := range d.NodeMap {
		for nd2, v := range nd1.WeightTo {
			one := Edge{}
			one.Src = nd1
			one.Dst = nd2
			one.Weight = v
			rs = append(rs, one)
		}
	}
	return rs
}
