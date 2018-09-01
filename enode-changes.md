This PR is the long-announced change to clean up p2p node information
handling throughout the codebase.

The decision to submit all these changes in a single PR was not easy. An
alternative would have been to create a temporary backwards-compatibility
layer which would have been in place while all packages were adapted. I
decided to follow through with the big PR because API requirements were not
clear when I started working on this. The new "p2p/enode" package changed
many times during this process.

Porting to `enode.Node` brings support for ENR features to every place that
deals with node information, but more work will be needed to actually start
relaying node records across protocols.

### The new p2p/enode package

The new package defines a type `Node`, which wraps an ENR and provides
access to common information such as IP address and ports as methods.

This package is the new home of the enode URL parser and the node database.

- TODO: Node database expirer is launched on first write.

### Changes to existing code

Type changes throughout the codebase:

    discover.Node -> enode.Node
    discover.NodeID /* 64B pubkey */ -> enode.ID /* 32B hash */

Changes to individual packages are listed below.

Package p2p/enr:

- Record signature handling is changed significantly. The identity scheme
  registry is removed and acceptable schemes must be passed to any method
  that needs identity. This means records must now be validated explicitly
  after decoding.
  
  The enode API is designed to make signature handling easy and safe: most
  APIs around the codebase work with enode.Node, which is a wrapper around a
  valid record. Going from `enr.Record` to `enode.Node` requires a valid
  signature.

Package p2p/discover: 

- The Kademlia table is now based on `enode.Node`. More work is needed to
  make it deal with metadata updates (i.e. handling ENR sequence numbers).
- A couple of API changes were needed to work around the fact that the V4
  protocol can't look up nodes by their hashed ID:
  - `Table.Resolve` now takes a node instead of `NodeID`.
  - `Table.Lookup` is unexported. Now we have `LookupRandom`, which is
     the only thing needed for package p2p.
    
Package p2p: 

- Bootnode lists use `[]*enode.Node`.
- `dialstate` uses interface to get random `[]*enode.Node` instead of
  invoking discovery lookup directly.
- `Peer` has a new method `Node` to retrieve the node record of the peer.
  For now, this method returns a record created from RLPx info because
  devp2p doesn't exchange ENR. I have a couple of ideas around changing
  this, maybe we could resolve the most recent record from the database
  later.

Package les:

- `serverPool` has the most changes because it's a database dealing with
  nodes and their addresses :). IMHO we should change it to use the actual
  node database, but that's not implemented in this PR. For now the
  database format is unchanged and `enode.Node` instances are reified from
  the `(pubkey, ip, port)` triple stored by this format.
- Other changes are related to the node ID type change.

Package eth:

- Very few changes were required, mostly just adaptation to the node ID
  change. Node IDs are represented as `string`, leaving most packages in
  the subtree unaffected by the change.

Package whisper/...:


