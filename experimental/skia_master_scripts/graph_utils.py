# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Utilities for working with graphs. """


class GraphHasACycleError(Exception):
  """Raised when an operation which requires a DAG is attempted on a Graph with
  a cycle."""
  pass


class NoSuchNodeError(Exception):
  """Raised when attempting to access a Node with an unknown ID."""
  def __init__(self, node_id):
    self._id = node_id

  def __str__(self):
    return 'No such Node: %d' % self._id


class Graph():
  """Abstract Graph class for expressing relationships between objects.
  Supports some graph operations like topological sort."""
  def __init__(self):
    self._graph = {}

  class Node():
    """Represents a Node in a Graph."""
    def __init__(self, id, ptr=None, in_edges=None, out_edges=None):
      self._id = id
      self._ptr = ptr
      self._in_edges = in_edges or []
      self._out_edges = out_edges or []

    def add_in_edge(self, id):
      """Adds an incoming edge to this node. The caller is responsible for
      making sure that the in_edges and out_edges between Nodes match up."""
      self._in_edges.append(id)

    def add_out_edge(self, id):
      """Adds an outgoing edge from this node. The caller is responsible for
      making sure that the in_edges and out_edges between Nodes match up."""
      self._out_edges.append(id)

    def remove_in_edge(self, id):
      """Removes an in_edge to this Node. The caller is responsible for making
      sure that the in_edges and out_edges between Nodes match up."""
      self._in_edges.remove(id)

    def remove_out_edge(self, id):
      """Removes an out_edge from this Node. The caller is responsible for
      making sure that the in_edges and out_edges between Nodes match up."""
      self._out_edges.remove(id)

    @property
    def id(self):
      """A unique identifier for this Node."""
      return self._id

    @property
    def ptr(self):
      """Pointer to the object (if any) stored in this Node."""
      return self._ptr

    @property
    def in_edges(self):
      """List of IDs of Nodes which have edges coming into this Node."""
      return self._in_edges

    @property
    def out_edges(self):
      """List of IDs of Nodes to which this Node has edges."""
      return self._out_edges

  def add_node(self, obj=None):
    """Adds a node with a reference to the given object.

    Args:
        obj: any object; may be None.

    Returns:
        The ID for the created Node.
    """
    id = len(self._graph)
    self._graph[id] = Graph.Node(id, obj)
    return id

  def add_edge(self, from_id, to_id):
    """Adds an edge from one node to another.

    Args:
        from_id: ID of the node where the edge begins.
        to_id: ID of the node where the edge terminates.

    Raises:
        NoSuchNodeError if either Node ID does not exist.
    """
    self._get_node(from_id).add_out_edge(to_id)
    self._get_node(to_id).add_in_edge(from_id)

  def _get_node(self, id):
    """Helper function for accessing Nodes.

    Args:
        id: The ID of the desired Node.

    Returns:
        The Node with the requested ID.

    Raises:
        NoSuchNodeError if the Node ID does not exist.
    """
    try:
      return self._graph[id]
    except KeyError:
      raise NoSuchNodeError(id)

  def __getitem__(self, id):
    """Accessor for the object held by the Node with the given ID.

    Args:
        id: The ID of a Node.

    Returns:
        A reference to the object held by the Node.

    Raises:
        NoSuchNodeError if the Node ID does not exist.
    """
    return self._get_node(id).ptr


  def children(self, id):
    """Get the Nodes to which the given Node has edges.

    Args:
        id: ID of the Node in question.

    Returns:
        A list of Node IDs.
    """
    return self._get_node(id).out_edges

  def parents(self, id):
    """Get the Nodes which have edges to the given Node.

    Args:
        id: ID of the Node in question.

    Returns:
        A list of Node IDs.
    """
    return self._get_node(id).in_edges

  def topological_sort(self):
    """Returns a sorted list of the Nodes in the graph such that all edges
    point to Nodes further down in the list.

    Returns:
        List of Node IDs in topologically-sorted order.
    """
    # We make copy of the graph, so that we can manipulate it without changing
    # the original.
    graph = {}

    # A list which will contain the node id's in topologically-sorted order.
    sorted_nodes = []

    # A list of node id's of nodes who have no incoming edges.
    no_incoming_edges = []

    # Copy the graph and find all nodes with no incoming edges.
    for id in self._graph.keys():
      node = self._graph[id]
      graph[id] = Graph.Node(id=node.id,
                             ptr=node.ptr,
                             in_edges=list(node.in_edges),
                             out_edges=list(node.out_edges))
      if not node.in_edges:
        no_incoming_edges.append(id)

    # Sort the nodes topologically.
    while no_incoming_edges:
      current_node = no_incoming_edges.pop()
      sorted_nodes.append(current_node)
      # Visit each child node.
      while graph[current_node].out_edges:
        # Remove the edge from current_node to child_node.
        child_node = graph[current_node].out_edges.pop()
        graph[child_node].in_edges.remove(current_node)

        # If child_node has no more incoming edges, add it to no_incoming_edges.
        if not graph[child_node].in_edges:
          no_incoming_edges.append(child_node)

    # If there are still edges in the graph, then there is a cycle.
    for node in graph.keys():
      if graph[node].in_edges or graph[node].out_edges:
        raise GraphHasACycleError(
            'Graph contains a cycle involving node %d' % node)

    return sorted_nodes

  def has_cycle(self):
    """Returns True iff the Graph contains a cycle.

    Returns:
        boolean; True if the Graph contains a cycle, or False otherwise.
    """
    try:
      self.topological_sort()
      return False
    except GraphHasACycleError:
      return True
