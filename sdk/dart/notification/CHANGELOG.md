## 2.0.0

- Regenerated with protoc_plugin 25.1 / connect-dart 2.0; requires protobuf ^6.1.0, connectrpc ^2.0.0, Dart ^3.7. WKTs now come from package:protobuf.
- Breaking: the vendored `google/protobuf` well-known types (`any`, `duration`, `field_mask`, `struct`, `timestamp`) are removed; the barrel re-exports `Struct` and `Timestamp` from `package:protobuf/well_known_types`.
- Breaking: the empty `.pbserver.dart` files for non-service protos are removed (protoc_plugin 25 no longer emits them).
- Requires `antinvestor_api_common` ^2.0.0.

## 1.53.0

- Generated Dart SDK from proto definitions

