Changes enacted 11Nov20

This PR adds an extra guarantee to NodeStateMachine: it ensures that all
immediate effects of a certain change are processed before any subsequent
effects of any of the immediate effects on the same node. In the original
version, if a cascaded change caused a subscription callback to be called
multiple times for the same node then these calls might have happened in a
wrong chronological order.

For example:

- a subscription to flag0 changes flag1 and flag2
- a subscription to flag1 changes flag3
- a subscription to flag1, flag2 and flag3 was called in the following order:

   [flag1] -> [flag1, flag3]
   [] -> [flag1]
   [flag1, flag3] -> [flag1, flag2, flag3]

This is because the tree of changes was traversed in a "depth-first
order". Now it is traversed in a "breadth-first order"; each node has a
FIFO queue for pending callbacks and each triggered subscription callback
is added to the end of the list. The already existing guarantees are
retained; no SetState or SetField returns until the callback queue of the
node is empty again. Just like before, it is the responsibility of the
state machine design to ensure that infinite state loops are not possible.
Multiple changes affecting the same node can still happen simultaneously;
in this case the changes can be interleaved in the FIFO of the node but the
correct order is still guaranteed.

A new unit test is also added to verify callback order in the above scenario.
