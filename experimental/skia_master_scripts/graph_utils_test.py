# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Tests for the graph_utils module. """


import graph_utils
import unittest


def _SimpleCycleGraph():
  """ Returns a graph with a single node with an edge to itself. """
  g = graph_utils.Graph()
  id = g.add_node()
  g.add_edge(id, id)
  return g


def _ComplexGraph(add_cycle=False):
  """ Returns a graph with several nodes, optionally with a cycle. """
  g = graph_utils.Graph()
  n1 = g.add_node()
  n2 = g.add_node()
  n3 = g.add_node()
  n4 = g.add_node()
  n5 = g.add_node()
  g.add_edge(n1, n2)
  g.add_edge(n1, n3)
  g.add_edge(n1, n4)
  g.add_edge(n2, n3)
  g.add_edge(n3, n5)
  g.add_edge(n4, n5)
  if add_cycle:
    g.add_edge(n5, n2)
  return g


class TestGraphUtils(unittest.TestCase):
  """ Tests for the graph_utils module. """
  # TODO(borenet): These tests only verify that graph_utils doesn't crash and
  # that it correctly identifies cycles. We should add correctness tests for
  # topological_sort.

  def test_TopologicalSortEmpty(self):
    g = graph_utils.Graph()
    g.topological_sort()

  def test_TopologicalSortSingleNode(self):
    g = graph_utils.Graph()
    g.add_node()
    g.topological_sort()

  def test_TopologicalSortSimpleCycle(self):
    g = _SimpleCycleGraph()
    self.assertRaises(Exception, g.topological_sort)

  def test_TopologicalSortComplex(self):
    g = _ComplexGraph()
    g.topological_sort()

  def test_TopologicalSortComplexCycle(self):
    g = _ComplexGraph(add_cycle=True)
    self.assertRaises(Exception, g.topological_sort)

  def test_HasCycleEmpty(self):
    g = graph_utils.Graph()
    self.assertFalse(g.has_cycle())

  def test_HasCycleSingleNode(self):
    g = graph_utils.Graph()
    g.add_node()
    self.assertFalse(g.has_cycle())

  def test_HasCycleSimpleCycle(self):
    g = _SimpleCycleGraph()
    self.assertTrue(g.has_cycle())

  def test_HasCycleComplex(self):
    g = _ComplexGraph()
    self.assertFalse(g.has_cycle())

  def test_HasCycleComplexCycle(self):
    g = _ComplexGraph(add_cycle=True)
    self.assertTrue(g.has_cycle())


if __name__ == '__main__':
  unittest.main()
