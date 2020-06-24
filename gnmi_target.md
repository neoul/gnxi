# gNMI Specification

## Path message

### `element` field of Path message

- Deprecated, Not used.

### `target` field of Path message

- This field is optional for clients. When set in the prefix in a request, GetRequest, SetRequest or SubscribeRequest, the field MUST be reflected in the prefix of the corresponding GetResponse, SetResponse or SubscribeResponse by a server.
- It is not used in gNMI server.

### `origin` field of Path message

- It MAY be used to disambiguate the path if necessary. For example, the origin may be used to indicate which organization defined the schema to which the path belongs.
- It would be the oranization defined YANG data module for the path message if used.

## Notification message

- **gnmi_target** supports `timestamp`, `prefix`, `update` and `delete` field in Notification.
- **gnmi_target** does **NOT** support `alias`.
- The creator of a Notification message MUST include the timestamp field. All other fields are optional.

## Update message

The field in the Notification message

### `duplicates` field of Update message

- A counter value that indicates the number of coalesced duplicates. If a client is unable to keep up with the server, coalescion can occur on a per update (i.e., per path) basis such that the server can discard previous values for a given update and return only the latest. In this case the server SHOULD increment a count associated with the update such that a client can detect that transitions in the state of the path have occurred, but were suppressed due to its inability to keep up.
- **gnmi_target** SHOULD increment the duplicates field if the state of the path was changed.

## Subscrbe RPC

- **Receive ongoing updates**: Receives initial set of updates and then subsequent updates as change.
- **Receive a single view**: The snapshot of the data tree in the target (ONCE, POLL)
- Subscriptions are set once, and subsequently not modified by a client.

### A difference to Get RPC

The target does not need to coalesce values into a single snapshot view, or create an in-memory representation of the subtree at the time of the request, and subsequently transmit this entire view to the client. By default a target MUST NOT aggregate values within an update message.

### Subscription mode

The mode of each subscription determines the triggers for updates for data sent from the target to the client.

- ONCE
- STREAM [ON_CHANGE, SAMPLE, TARGET_DEFINED]
- POLL

#### ONCE

- The RPC session must be closed following the initial response generation with the relevant status code.

#### STREAM

- ON_CHANGE (On Change)
- SAMPLE (Sampled)
- TARGET_DEFINED (Target Defined)

#### POLL

### Cancel subscription

Subscriptions are created for a set of paths - which cannot be modified throughout the lifetime of the subscription. In order to cancel a subscription, the client cancels the Subscribe RPC associated with the subscription, or terminates the entire gRPC session.

> Need to check how to cancel Subscribe RPC

### SubscribeRequest

- `subscribe`, `poll`, `aliases` 중 하나만 설정가능 (`oneof` type in protobuf)
  - `SubscribeRequest{poll:true}`, `SubscribeRequest{subscribe:...}`, `SubscribeRequest{aliases}`
- `allow_aggregation`: 해당 subscription의 telemetry update가 다른 update와 combine 될 수 있음을 설정. (보통 사용 X)

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

### REQUIREMENT for Path Aliases

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

### REQUIREMENT for ModelData

- [ ] If the client specifies a set of models in a Get or Subscribe RPC, the target MUST NOT utilize data tree elements that are defined in schema modules outside the specified set.
- [ ] In addition, where there are data tree elements that have restricted value sets (e.g., enumerated types), and the set is extended by a module which is outside of the set, such values MUST NOT be used in data instances that are sent to the client.
- [ ] Where there are other elements of the schema that depend on the existence of such enumerated values, the target MUST NOT include such values in data instances sent to the client.
