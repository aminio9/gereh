# Events platform package

This package will contain only technical Kafka primitives:

- Event envelope serialization
- Producer and consumer construction
- Transactional inbox helpers
- Trace propagation
- Retry classification

Business events remain owned by their producing service and are defined under `contracts/events`.
