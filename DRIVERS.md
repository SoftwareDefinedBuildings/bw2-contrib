# Drivers

A guide on how to construct BW2 drivers.

Drivers abstract away the details of communicating to underlying hardware and
services to present a uniform interface.

This document should be consumed after reading up on Bosswave services (docs coming soon).

## URIs

Drivers and services in BOSSWAVE publish and subscribe to URIs.

In this section we establish a set of idioms for URI construction for these drivers.
We can divide URIs into several components:

```
<baseuri>/<service>/<instance>/<interface>/{signal,slot}/<name>
```

* **`baseuri`**: the base URI, starting with a namespace (e.g. `410.dev`) and
  probably some additional subdivision, e.g. `410.dev/sodahall/drivers`. This
  is the URI information provided to the driver during its initial
  configuration because this describes where in the administrative hierarchy
  the driver is deployed.  The driver will generate all URIs from this point
  starting with the `service` component below.

* **`service`**: the name of the service, using the `s.` notation, so a "LIFX
  driver" might be named `s.lifx`. The service name will be dictated by the
  driver.

  * **TODO**: we want a list of all available service names and links to the drivers

* **`instance`**:  combined with `interface` below, describes a unique instance
  of an interface. This is not the appropriate place to attach descriptive
  metadata, so the `instance` name should just be enough to distinguish between
  instances.  For example, a plug strip service (e.g. `s.plugstrip`) might have
  multiple plugs that each present a binary actuator interface (e.g.
  `i.binary_actuator`). In this case, we might name the instances by their plug
  number: `0`, `1`, etc.

* **`interface`**: the name of the interface uses the `i.` notation, so a
  hue-saturation-brightness interface for a light could be named `i.hsb-light`.
  An interface defines a set of `signal`s and `slot`s for publishing and
  consuming data respectively.

  * **TODO**: define interfaces somewhere

* **signal**s and **slot**s: URI endpoints are either a **signal** (output) or a **slot** (input).
  A signal emits new readings and state from the service. Processes subscribe to a signal.
  Signals can be used for advertising state changes, new sensor readings, etc.

  A slot is a URI to which the service subscribes and is how other processes can send
  data to a service.

* **`name`**: this is the name of the signal or slot on the interface. The
  combination of `signal`/`slot` and `name` is defined by the interface.

## Metadata

Metadata is descriptive data attached to a service, an instance, an interface, or a signal/slot.
Services will persist data at special BOSSWAVE URIs.

All resources in BOSSWAVE are identified with some URI that acts as a kind of
"pointer": this is true both for topics acting as data transport channels
between publishers and subscribers (as with the signals and slots above) as
well as *persisted data*. 

Any message published on a URI will be delivered to all subscribers, but in
some cases a process may want to see whatever message was last published on a
URI without having to wait for something to publish on that URI. Persisted
messages "live" on a URI and can be retrieved using a [BOSSWAVE query](https://godoc.org/gopkg.in/immesys/bw2bind.v5#BW2Client.Query)
(not a subscription).

By convention, metadata attached to a URI `<u>` should be persisted at
`<u>/!meta/<keyname>`. ("!" has no special meaning). Because the persisted
objects are full messages, they can contain lists of Payload Objects of various
types.

In a service URI, we expect to place metadata at the following locations (`^`)
```
<baseuri>/<service>/<instance>/<interface>/{signal,slot}/<name>
                   ^          ^           ^                    ^
               service MD  instance MD  iface MD             signal/slot MD
```

## Permissions

## Deployment

## Archiving
