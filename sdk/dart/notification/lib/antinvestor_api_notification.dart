/// Dart client library for Ant Investor Notification Service.
///
/// Provides Notification service functionality using Connect RPC protocol.
library;

// Notification service
export 'src/notification/v1/notification.pb.dart';
export 'src/notification/v1/notification.pbenum.dart';
export 'src/notification/v1/notification.pbjson.dart';
export 'src/notification/v1/notification.connect.client.dart';
export 'src/notification/v1/notification.connect.spec.dart';

// Common types
export 'src/common/v1/common.pb.dart';
export 'src/common/v1/common.pbenum.dart';
export 'package:protobuf/well_known_types/google/protobuf/struct.pb.dart';
export 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart';
