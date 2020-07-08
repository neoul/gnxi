# gNMI Specification

- [gNMI Specification](#gnmi-specification)
  - [Path message](#path-message)
  - [TypedValue message](#typedvalue-message)
  - [Encoding message](#encoding-message)
  - [Notification message](#notification-message)
  - [Update message](#update-message)
  - [gNMI Alias Control (Optional)](#gnmi-alias-control-optional)
    - [Path Aliases in SubscribeRequest](#path-aliases-in-subscriberequest)
    - [Target-defined Aliases in SubscribeResponse](#target-defined-aliases-in-subscriberesponse)
  - [gNMI ModelData Control](#gnmi-modeldata-control)
  - [gNMI Extensions [TBD]](#gnmi-extensions-tbd)
  - [Get RPC](#get-rpc)
  - [Set RPC](#set-rpc)
  - [Subscrbe RPC](#subscrbe-rpc)
    - [SubscribeRequest message](#subscriberequest-message)
    - [SubscribeResponse message](#subscriberesponse-message)

## [Path message](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-path-conventions.md)

Paths are represented according to gNMI Path Conventions, which defines a structured format for path elements, and any associated key values.

```yaml
Path:
  element(string):
  origin(string):
  elem(PathElem): # list
    name(string):
    key(map<string,string>)
    target(string):
```

- [ ] `element(string)` is deprecated, Not used in gnmi_target.
- [ ] `target(string)` This field is optional for clients. When set in the prefix in a request, GetRequest, SetRequest or SubscribeRequest, the field MUST be reflected in the prefix of the corresponding GetResponse, SetResponse or SubscribeResponse by a server.
- [ ] `origin(string)` It MAY be used to disambiguate the path if necessary. For example, the origin may be used to indicate which organization defined the schema to which the path belongs.
- [ ] `origin(string)` MUST be the oranization defined YANG data module for the path message if used.

## TypedValue message

The value of a data node (or subtree) is encoded in a `TypedValue` message as a oneof field to allow selection of the data type by setting exactly one of the member fields.

- scalar types
- additional types used in some schema languages
- structured data types (e.g., to encode objects or subtrees)

```yaml
TypedValue:
  string_val(string):             # String value.
  int_val(int64):                 # Integer value.
  uint_val(uint64):               # Unsigned integer value.
  bool_val(bool):                 # Bool value.
  bytes_val(bytes):               # Arbitrary byte sequence value.
  float_val(float):               # Floating point value.
  decimal_val(Decimal64):         # Decimal64 encoded value.
    digits(int64):
    precision(uint32):
  leaflist_val(ScalarArray):      # Mixed type scalar array value.
    element(TypedValue):
  any_val(google.protobuf.Any):   # protobuf.Any encoded bytes.
  json_val(bytes):                # JSON-encoded text.
  json_ietf_val(bytes):           # JSON-encoded text per RFC7951.
  ascii_val(string):              # Arbitrary ASCII text.
  proto_bytes(bytes):             # github.com/openconfig/reference/blob/master/rpc/gnmi/protobuf-vals.md
```

- [ ] A TypedValue element within `leaflist_val(ScalarArray)`. The type of each element MUST be a scalar type (i.e., one of the scalar types or Decimal64).

## [Encoding message](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#23-structured-data-types)

Encoding defines the value encoding formats that are supported by the gNMI protocol. These encodings are used by both the client (when sending Set messages to modify the state of the target) and the target when serializing data to be returned to the client (in both Subscribe and Get RPCs).

```yaml
Enconding:
  - JSON(0)      # json_val in TypedValue
  - BYTES(1)     # bytes_val in TypedValue
  - PROTO(2)     # any_val in TypedValue
  - ASCII(3)     # ascii_val in TypedValue
  - JSON_IETF(4) # json_ietf_val in TypedValue
```

- [ ] When structured data is sent by the client or the target in an Update message, it MUST be serialized according to one of the supported encodings listed in the Encoding enumeration.
- [ ] All JSON values MUST be valid JSON.
- [ ] JSON encoded data MUST conform with the rules for JSON serialisation described in RFC7159.
- [ ] JSON_IETF encoded data MUST conform with the rules for JSON serialisation described in RFC7951. Data specified with a type of JSON MUST be valid JSON, but no additional constraints are placed upon it.
- [ ] An implementation MUST NOT serialise data with mixed JSON and JSON_IETF encodings.
- [ ] Both the client and target MUST support the JSON encoding as a minimum.
- [ ] Another encoding **[TBD]**

## Notification message

When a target wishes to communicate data relating to the state of its internal database to an interested client, it does so via means of a common Notification message.

Notification is a re-usable message that is used to encode data from the
target to the client. A Notification carries two types of changes to the data
tree:

- Deleted values (`delete`) - a set of paths that have been removed from the data tree.
- Updated values (`update`) - a set of path-value pairs indicating the path whose value has changed in the data tree.

```yaml
GetResponse:
  notification(Notification):
    timestamp(int64):
    prefix(Path):
    alias(string):
    update(Update): # list
    delete(Path): # list
    atomic(boo): # no description how to use.
SubscribeResponse:
  notification(Notification): ...
```

> There is no introduction for `atomic` field.

- [ ] gnmi_target supports `timestamp`, `prefix`, `update` and `delete` field in Notification.
- [ ] The creator of a Notification message MUST include the timestamp field. All other fields are optional.
- [ ] Timestamp values MUST be represented as the number of nanoseconds since the Unix epoch (January 1st 1970 00:00:00 UTC). The value MUST be encoded as a signed 64-bit integer (int64)[^1].

## Update message

A list of update messages that indicate changes in the underlying data of the target. Both modification and creation of data is expressed through the update message.

```yaml
Update:
  path(Path):
  value(Value): # deprecated, not used
  val(TypedVal):
  duplicates(uint32):
```

- [ ] When structured data is sent by the client or the target in an `Update` message, it MUST be serialized according to one of the supported encodings listed in the `Encoding` enumeration.
- [ ] `duplicates(uint32)` is a counter value that indicates the number of coalesced duplicates. If a client is unable to keep up with the server, coalescion can occur on a per update (i.e., per path) basis such that the server can discard previous values for a given update and return only the latest. In this case the server SHOULD increment a count associated with the update such that a client can detect that transitions in the state of the path have occurred, but were suppressed due to its inability to keep up.
- [ ] gnmi_target SHOULD increment the duplicates field if the state of the path was changed.

## gNMI Alias Control (Optional)

```yaml
Alias:
  SubscribeRequest:
    aliases(AliasList): # list
      alias(Alias):
        path(Path):
        alias(string):
    subscribe(SubscriptionList):
      use_aliases(bool):
  SubscribeResponse:
    update(Notification):
      timestamp(int64):
      prefix(Path):
      alias(string):
  GetRequest: noting relative to alias.
  GetResponse:
    notification(Notification):
      timestamp(int64):
      prefix(Path):
      alias(string):
```

### Path Aliases in SubscribeRequest

- The path aliases is used to reduces total message length by using aliases rather than using a complete representation of the path.
- An alias name MUST be prefixed with a `#` character.
- An alias name is deleted by empty alias corresponding the path.
- An alias doesn't refer to other aliases for single alias lookup.
- `use_aliases`: a boolean flag indicating whether the client accepts target aliases via the subscription RPC.
- By default, path aliases created by the target are not supported.

### [Target-defined Aliases in SubscribeResponse](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#3522-target-defined-aliases-within-a-subscription)

- Where the `use_aliases` field of a SubscriptionList message has been set to true, a target MAY create aliases for paths within a subscription.
- A target-defined alias MUST be created separately from an update to the corresponding data item(s).

- [ ] gnmi_target MAY support Path Aliases upon Subscribe RPC operation.
- [ ] gnmi_target MUST enable Path Aliases if `use_aliases` is set to true on `SubscribeRequest`.
- [ ] gnmi_target MUST update the internal AliasList if `aliases(AliasList)` presents on `SubscribeRequest`.
- [ ] gnmi_target MUST change `prefix(Path)` to `alias(string)` on `SubscribeResponse` according to the internal AliasList.
- [ ] gnmi_target MAY support Target-defined Aliases if `use_aliases` is set to true on `SubscribeRequest`.
- [ ] gnmi_target MUST update the internal AliasList according to the subscription path of `SubscribeRequest` automatically if Target-defined Aliases supported.

## gNMI ModelData Control

- The ModelData message describes a specific model that is supported by the target and used by the client.
- The fields of the ModelData message identify a data model registered in a model catalog, as described in [MODEL_CATALOG_DOC].
  - https://datatracker.ietf.org/doc/draft-openconfig-netmod-model-catalog/
  - openconfig-catalog-types.yang, openconfig-module-catalog.yang
- The combination of name, organization, and version uniquely identifies an entry in the model catalog.

```yaml
ModelData:
  CapabilityResponse:
    supported_models(ModelData): # list
      name(string):
      oranization(string):
      version(string): # openconfig-version
  GetRequest:
    use_models(ModelData): # list
      name(string):
      oranization(string):
      version(string): # openconfig-version
  SubscribeRequest:
    subscribe(SubscriptionList): # list
      use_models(ModelData):
        name(string):
        oranization(string):
        version(string): # openconfig-version
```

- [ ] If the client specifies a set of models in a Get or Subscribe RPC, the target MUST NOT utilize data tree elements that are defined in schema modules outside the specified set.
- [ ] In addition, where there are data tree elements that have restricted value sets (e.g., enumerated types), and the set is extended by a module which is outside of the set, such values MUST NOT be used in data instances that are sent to the client.
- [ ] Where there are other elements of the schema that depend on the existence of such enumerated values, the target MUST NOT include such values in data instances sent to the client.

## [gNMI Extensions](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-extensions.md) [TBD]

- Extensions to gNMI define a means to add new payload to gNMI RPCs for these use cases without requiring changes in the core protocol specification.
- gNMI extensions are defined to be carried within the `extension` field of each top-level message of the gNMI RPCs.
- gNMI extensions are implemented directly within the request and response messages to allow logging, debugging, or tracing frameworks to capture their contents, and avoid the fragility of carrying extension information in metadata.
- The Extension message is defined within the `gnmi_ext.proto` specification.

## [Get RPC](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#33-retrieving-snapshots-of-state-information)

## [Set RPC](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#34-modifying-state)

## [Subscrbe RPC](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#35-subscribing-to-telemetry-updates)

When a client wishes to receive updates relating to the state of data instances on a target, it creates a subscription via the Subscribe RPC.

### SubscribeRequest message

A SubscribeRequest message is sent by a client to request updates from the target for a specified set of paths.

```yaml
SubscribeRequest: # !!oneof
  subscribe(SubscriptionList):
    prefix(Path):
    subscription(Subscription): # list
      path(Path): ...
      mode(SubscriptionMode): # subscription mode
        - TARGET_DEFINED(0)
        - ON_CHANGE
        - SAMPLE
      sample_interval(uint64): # in nanoseconds
      suppress_redundant(bool):
      heartbeat_interval(uint64): # in nanoseconds
    use_aliases(bool):
    qos(QOSMarking):
      marking(unit32):
    mode(Mode): # telemetry update mode
      - SREAM(0)
      - ONCE
      - POLL
    allow_aggregation(bool):
    use_models(ModelData): ...
    encoding(Encoding): [JSON(0), BYTES, PROTO, ASCII, JSON_IETF(4)]
    updates_only(bool):
  poll(Poll):
  aliases(AliasList): # list
    alias(Alias):
      path(Path): ...
      alias(string):
```

- [ ] gnmi_target MUST receive and parse `SubscribeRequest` messages from gRPC channel.
- [ ] gnmi_target MUST send `SubscribeResponse`s as telemetry updates according to the received `SubscribeRequest`.
- [ ] gnmi_target MUST support three types of telemetry mode (one-off, poll-only and streaming).
  - [ ] These types are set to `mode(Mode)` field with one of the three values `[STREAM, ONCE, POLL]` in `SubscribeRequest`.
- [ ] **STREAM Subscriptions**: Stream subscriptions are long-lived subscriptions which continue to transmit updates relating to the set of paths that are covered within the subscription indefinitely.
  - [ ] Upon telemetry configuration (`SubscribeRequest` reception), gnmi_target MUST send `SubscribeResponse` messages periodically according to [3.5.1.5.2 STREAM Subscriptions](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#35152-stream-subscriptions) for each path configured.
  - [ ] `mode(SubscriptionMode)` is used to decide the period of the telemetry update transmission. gnmi_target MUST generate `SubscribeResponse` messages according to the subscription mode.
    - [ ] On TARGET_DEFINED mode, gnmi_target MUST determine the best type of subscription to be created on a per-leaf basis.
    - [ ] On ON_CHANGE mode, gnmi_target MUST send the telemetry updates when the value of the data item changes.
    - [ ] On SAMPLE mode, gnmi_target MUST generate the telemetry updates along with `sample_interval(uint64)`.
  - [ ] `sample_interval(uint64)` is the rate of sampling. If it is set to 0, send the data with the lowest interval possible for the target.
  - [ ] If `suppress_redundant(bool)` is set to true, gnmi_target SHOULD generate a telemetry update message with the values have been changed. This field is available on mode{SAMPLE}.
  - [ ] If `heartbeat_interval(uint64)` is set, gnmi_target MUST generate a telemetry update every hearbeat_interval regardless of the data change. This field is available on mode{ON_CHANGE} or mode{SAMPLE}.
- [ ] **POLL Subscriptions**: Polling subscriptions are used for on-demand retrieval of data items via long-lived RPCs.
  - [ ] POLL Subscription is enabled when gnmi_target receive a `SubscribeRequest` message containing an empty `poll(Poll)` message.
  - [ ] On reception of such a message, gnmi_target MUST generate updates for all the corresponding paths within the `subscribe(SubscriptionList)`.
  - [ ] Updates MUST be generated according to [3.5.2.3 Sending Telemetry Updates](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#3523-sending-telemetry-updates)
- [ ] **ONCE Subscriptions**: A subscription operating in the ONCE mode acts as a single request/response channel. The target creates the relevant update messages, transmits them, and subsequently closes the RPC.
  - [ ] ONCE Subscription MUST start on the reception of `SubscribeRequest` message with the `mode(Mode)` field set to `ONCE` and then the `SubscribeResponse` messages corresponding to the subscription MUST be transmitted to the client.
  - [ ] **ONCE Subscription termination**: ~~Following the transmission of all updates which correspond to data items within the set of paths specified within the subscription list,~~ After all Updates (`SubscribeResponse`) are transmitted, a `SubscribeResponse` message with the `sync_response` field set to true MUST be transmitted, and the RPC via which the `SubscribeRequest` was received MUST be closed.
  - [ ] If `updates_only` is set in the SubscribeRequest message, only a `SubscribeResponse(sync_resonse)` will be sent for a `ONCE` or `POLL` mode.
- [ ] **Cancel subscription**: Subscriptions are created for a set of paths - which cannot be modified throughout the lifetime of the subscription. In order to cancel a subscription, the client cancels the Subscribe RPC associated with the subscription, or terminates the entire gRPC session.
  - Need to check how to cancel Subscribe RPC (https://github.com/grpc/grpc-java/issues/3095)
- [ ] **Subscription aggregation**:
  - `allow_aggregation`: 해당 subscription의 telemetry update가 다른 update와 combine 될 수 있음을 설정. (보통 사용 X)
- [ ] **A difference to Get RPC**: The target does not need to coalesce values into a single snapshot view, or create an in-memory representation of the subtree at the time of the request, and subsequently transmit this entire view to the client. By default a target MUST NOT aggregate values within an update message.
- [ ] **updates_only(bool)**: An optional field to specify that only updates to current state should be sent to a client.
  - [ ] If set, the initial state is not sent to the client but rather only the sync message followed by any subsequent updates to the current state.
  - [ ] For ONCE and POLL modes, this causes the server to send only the sync message (Sec. 3.5.2.3), before proceeding to process poll requests (in the case of POLL) or closing the RPC (in the case of ONCE).
  - [ ] `updates_only(bool)`가 설정된 `SubscribeRequest`가 수신되면, gnmi_target은 inital updates (SubscribeResponse) message를 보내지 않고, initial updates complete message (SubscribeResponse{sync_response:true})만 전송한다. 그 뒤 변경된 data에 대해서 updates message (SubscribeResponse)를 보낸다.

> // https://github.com/openconfig/gnmi/issues/45 - QoSMarking seems to be deprecated

### SubscribeResponse message

A `SubscribeResponse` message is the telemetry updates that is transmitted by a target to a client over an established Subscribe RPC for a specified set of paths.

```yaml
SubscribeResponse: # !!oneof
  update(Notification): # not list!!
    timestamp(int64):
    prefix(Path):
    alias(string):
    update(Update): # list
    delete(Path): # list
    atomic(boo): # no description how to use.
  sync_response(bool):
  error(Error): # deprecated, not used
    ...
```

- [ ] `sync_response(bool)` MUST be set to true if all data values corresponding to the path(s) specified in the `SubscriptionList` of `SubscribeRequest` has been transmitted at least once.

- [ ] `update(Notification)` message MUST provide update values for a subscribed data entity as described in [3.5.2.3 Sending Telemetry Updates](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#3523-sending-telemetry-updates).
  - [ ] The `timestamp(int64)` field of the Notification message MUST be set to the time at which the value of the path that is being updated was collected from the underlying data source, or the event being reported on (in the case of ON_CHANGE occurred).
  - [ ] Where a leaf node's value has changed, or a new node has been created, an Update message specifying the path and value for the updated data item MUST be appended to the `update(Update)` field of the message.
  - [ ] **About delete telemetry update**: Where a node within the subscribed paths has been removed, the `delete(Path)` field of the `update(Notification)` message MUST have the path of the node that has been removed appended to it.
  - [ ] **About replace telemetry update**: To replace the contents of an entire node within the tree, gnmi_target MUST populate the `delete(Path)` field with the path of the node being removed, along with the new contents within the `update(Update)` field.
  - [ ] **Reporting init updates are completed**: When gnmi_target has transmitted the initial updates for all paths specified within the subscription, a SubscribeResponse message with the `sync_response` field set to true MUST be transmitted to the client to indicate that the initial transmission of updates has concluded. This provides an indication to the client that all of the existing data for the subscription has been sent at least once. For STREAM subscriptions, such messages are not required for subsequent updates. For POLL subscriptions, after each set of updates for individual poll request, a SubscribeResponse message with the `sync_response` field set to true MUST be generated.
  - [ ] In the case where the `updates_only` field in the `SubscribeRequest` message has been set, a `sync_response` is sent as the first message on the stream, followed by any updates representing subsequent changes to current state. For a `POLL` or `ONCE` mode, this means that only a `sync_resonse` will be sent. The `updates_only` field allows a client to only watch for changes, e.g. an update to configuration.
- [ ] `update(Notification)` field is also utilised when a target wishes to create an alias within a subscription, as described in [3.5.2.2 Target-defined Aliases within a Subscription](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#3522-target-defined-aliases-within-a-subscription).
