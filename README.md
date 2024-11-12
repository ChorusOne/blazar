<div align="center">
  <a href="#">
    <img src="https://github.com/user-attachments/assets/f5a48a09-b3ce-41d8-8fe9-3937da45038f" alt="Logo" width="192" height="192">
  </a>

  <h3 align="center">Blazar: Automatic Cosmos SDK Network Upgrades</h3>

  <p align="center">
    Life is too short to wait for the upgrade block height!
    <br />
    <br />
    <a href="#getting-started">Getting Started</a>
    ·
    <a href="#cli--rest-interface">CLI</a>
    ·
    <a href="#what-is-blazar">Web UI</a>
    ·
    <a href="#proxy-ui">Proxy UI</a>
    ·
    <a href="#slack-integration">Slack</a>
    ·
    <a href="#frequently-asked-questions">FAQ</a>
  </p>
</div>

## What is Blazar?
Blazar is a standalone application designed to automate network upgrades for [Cosmos SDK](https://github.com/cosmos/cosmos-sdk) based blockchain networks.

![Web UI](https://github.com/user-attachments/assets/3250a2d6-2bb7-4c1e-bc89-e8d15736300c)

## The Need for Blazar
At [Chorus One](https://chorus.one), we manage over 60 blockchain networks, many of which are part of the Cosmos Ecosystem. Each network has its own upgrade schedule, which can vary from monthly to bi-weekly, depending on the urgency of the upgrade and Cosmos SDK releases. Our 24/7 on-call team handles multiple upgrades weekly.

The upgrade process is generally straightforward but can be time-consuming. Here's how it typically works:
1. An upgrade is announced via a governance proposal or other communication channels (Discord, Telegram, Slack, Email, etc.).
2. The upgrade details specify the block height and the version network operators should use.
3. At the specified block height, the node halts, and operators must upgrade the binary and restart the node(s).
4. While waiting for consensus, operators often engage in progress updates on Discord.
5. Once the upgrade is successful, operators return to their regular tasks.

Blazar was created to automate this process, allowing our team to use their time more productively. It currently handles the majority of network upgrades for Cosmos Networks at Chorus One.

## Key Features
- **Upgrade Feeds:** Fetch upgrade information from multiple sources like "Governance", "Database", and "Local".
- **Upgrade Strategies:** Supports various upgrade scenarios, including height-specific and manually coordinated upgrades.
- **Pre and Post Upgrade Checks:** Automate checks like docker image existence, node and consensus status.
- **Stateful Execution:** Tracks upgrade stages to ensure consistent execution flow.
- **Cosmos SDK Gov/Upgrade Module Compliance:** Understands and respects the Cosmos SDK governance module upgrades.
- **Slack Integration:** Optional Slack notifications for every action Blazar performs.
- **Modern Stack:** Includes CLI, UI, REST, gRPC, Prometheus metrics, and Protobuf.
- **Built by Ops Team:** Developed by individuals with firsthand experience in node operations.

## Comparison to Cosmovisor
While many operators use [Cosmovisor](https://docs.cosmos.network/main/build/tooling/cosmovisor) with systemd services, this setup doesn't meet our specific needs. Instead of relying on GitHub releases, we [build our own binaries](https://handbook.chorus.one/node-software/build-process.html), ensuring a consistent build environment with Docker. This approach allows us to use exact software versions and generate precise build artifacts (e.g., libwasmvm.so).

Cosmovisor is designed to run as the parent process of a validator node, replacing node binaries at the upgrade height. However, this model isn't compatible with Docker Compose managed services. To address this, we developed Blazar as a more effective solution tailored to our setup.

**Note:** If you'd like Blazar to work with systemd services, contributions are welcome!

|                   	| Blazar                                     	| Cosmovisor                      	|
|-------------------	|--------------------------------------------	|---------------------------------	|
| Control plane     	| Docker                                     	| [Fork/Exec](https://docs.cosmos.network/main/build/tooling/cosmovisor#design)                       	|
| Upgrade mechanism 	| Image Tag Update                           	| Replace Binary                  	|
| Configuration     	| TOML (Blazar) + YAML (docker-compose.yml)  	| [Custom directory structure](https://docs.cosmos.network/main/build/tooling/cosmovisor#folder-layout) 	|
| Upgrade strategy  	| Gov, Coordinated, Uncoordinated           	| [Gov, Coordinated, Uncoordinated](https://docs.cosmos.network/main/build/tooling/cosmovisor#adding-upgrade-binary)   |
| Upgrade scope     	| Single, Multi-node*                        	| Single node                      	|
| Pre checks        	| :heavy_check_mark:                          | :heavy_check_mark: (preupgrade.sh)              	|
| Post checks       	| :heavy_check_mark:                          | :x:                              	|
| Metrics            	| :heavy_check_mark:                        	| :x:                              	|
| Notifications     	| :heavy_check_mark: (Slack)                 	| :x:                              	|
| UI + REST + RPC   	| :heavy_check_mark:                         	| :x:                              	|
| CLI               	| :heavy_check_mark:                          | :heavy_check_mark:               	|
| Upgrade Feeds     	| Governance, Database, Local                	| Governance, Local**                |

\* `DATABASE` registered upgrades are executed by multiple nodes feeding from the provider

\** For Cosmovisor everything looks as [if it was scheduled through governance](https://docs.cosmos.network/main/build/tooling/cosmovisor#detecting-upgrades)

## How Blazar Works
![Blazar Under the Hood](https://github.com/user-attachments/assets/1ecb0629-24bc-472f-a9cf-4d8d5e271a86)

Blazar constructs a prioritized list of upgrades from multiple providers and takes appropriate actions based on the most recent state. It retrieves block heights from WSS endpoints or periodic gRPC polls and triggers Docker components when the upgrade height is reached. Notifications are sent to logs and Slack (if configured).

In simple terms, Blazar performs the following steps:
1. **Upgrade List Construction:** Blazar compiles a unified list of upgrades from various providers (database, local, chain), resolving priorities based on the highest precedence.
2. **State Evaluation & Action:** The Blazar daemon reads this list in conjunction with the most recent state, taking relevant actions, such as performing a pre-upgrade check or finalizing the upgrade process.
3. **Block Height Detection:** The daemon tracks block heights via WSS endpoints or periodic gRPC polls.
4. **Upgrade Execution:** When the upgrade height is reached, the corresponding Docker components are executed.
5. **Notification Delivery:** Blazar sends notifications to logs and Slack (if configured).

While the logic is simple, it's important to understand the differences between the types of upgrades:
1. **Governance:** A coordinated upgrade initiated by chain governance, expected to be executed by all validators at a specified block height.
2. **Non Governance Coordinated:** An upgrade initiated by operators, not by the chain, but it is expected to occur at the same block height across all validators.
3. **Non Governance Uncoordinated:** An operator-initiated upgrade, independent of chain governance, that can be executed at any time.

NOTE: Blazar does one job and does it well, meaning you need one Blazar instance per Cosmos-SDK node.

NOTE: You are free to choose your upgrade proposal providers. An SQL database is not mandatory - you can opt to use the "LOCAL" provider or both simultaneously, depending on your needs.

## Getting Started
To use Blazar, first build the binary with the Go compiler, then deploy it on a host with Docker Compose installed.
```sh
$ apt-get install golang
$ apt-get install docker-compose
```

Configure and run Blazar:
```sh
$ cp blazar.sample.toml blazar.toml
$ make build
$ ./blazar run --config blazar.toml
```

### Requirements: Docker & Docker Compose
Blazar is designed to work with nodes configured and spawned via Docker Compose.

### CLI & REST Interface
Register or list upgrades using the CLI:
```sh
$ ./blazar upgrades list --host 127.0.0.1 --port 5678
... table with upgrades ...

$ ./blazar upgrades register --height "13261400" --tag '4.2.0' --type NON_GOVERNANCE_COORDINATED --source DATABASE --host 127.0.0.1 --port 5678 --name 'security upgrade'
```

Or use the REST interface:
```
curl -s http://127.0.0.1:1234/v1/upgrades/list
```

### Slack Integration
Track the upgrade process in a single Slack thread 🧵.

![Slack Notifications](https://github.com/user-attachments/assets/f59139e8-cdf9-4cd1-87bf-e5b1c0a667a7)

### Proxy UI
Blazar Proxy consolidates the latest updates from all Blazar instances. Here's how you can run it:
```
$ cp proxy.sample.toml proxy.toml
$ ./blazar proxy --config proxy.toml
```

![Proxy UI](https://github.com/user-attachments/assets/2b80e96e-5e9a-4d59-ae9e-8479c8fdee81)


## Frequently Asked Questions
<details>
  <summary>Why do I need to register a version tag separately?</summary>
  
Cosmos-SDK Software Upgrade Proposals don't explicitly specify the version you must upgrade to. It can be derived from the rich text data within the proposal, such as:
1. A link to the binary release (if present).
2. The proposal title.
3. The human-written text.

Currently, Blazar does not infer which version should be used. As a network operator, you must provide a version tag; otherwise, Blazar will skip the upgrade.
</details>

<details>
  <summary>What are the upgrade priorities, and why do I need them?</summary>

Consider a scenario where a network operator runs three nodes. The first node uses an image with a patch (e.g., PebbleDB support), while the other two run vanilla upstream images.

In this configuration, Blazar uses three upgrade sources:
* CHAIN (priority 1)
* DATABASE (priority 2)
* LOCAL (priority 3)

All three Blazar instances detect a new upgrade from CHAIN. The operator registers a new version in the DATABASE so that every instance knows what to pick up. However, one node requires a patched version. The network operator must register a new version in the LOCAL provider.

Now, the first node sees two different versions from two providers (DATABASE & LOCAL). Which one should it use?
**The one with the higher priority**

The end state on each Blazar node is:
1. Node 1 - v1.0.0-patched, priority 3
2. Nodes 2 & 3 - v1.0.0, priority 2

The same logic applies to upgrade entries and versions.
</details>

<details>
  <summary>What happens if I don't register a version tag for an upgrade?</summary>

Blazar will skip the upgrade.
</details>

<details>
  <summary>Blazar doesn't display any upgrades?</summary>

Blazar maintains its own state of all upgrades, which is periodically refreshed at the interval specified in your configuration. If you don't see the upgrades, it is likely that you need to wait for the given interval for Blazar to update the state.

NOTE: Adding a new version or upgrade via CLI/UI will trigger a state update.
</details>

<details>
  <summary>The upgrade governance proposal passed, but the upgrade is still in the 'SCHEDULED' state?</summary>

Blazar will change the upgrade state from 'SCHEDULED' to 'ACTIVE' when the voting period is over.
</details>

<details>
  <summary>What is the purpose of the 'force cancel' flag?</summary>

There are two ways to cancel an upgrade in Blazar. The standard method creates a `cancellation entry` in the provider storage, such as an SQL database, if no upgrade is registered. Otherwise, it updates the upgrade status field to `CANCELLED` for the upgrade with the highest priority.

Blazar periodically fetches and updates the list of upgrades at the interval specified in your configuration. But what if you need to cancel the upgrade immediately and can't wait for the next fetch? For such uncommon scenarios, you can use the `force cancel` mode, which sets the `CANCELLED` status directly in the Blazar state machine.

The force mode works per Blazar instance, so if you have, say, 3 nodes, you would need to force cancel all three via CLI/UI/RPC calls. If you use the `DATABASE` provider, you can simply cancel the upgrade for everyone, but you need to wait for Blazar to pick it up.

To simplify, think of the `force cancel` as the last line of defense. It is unlikely that you will need it, but it's there just in case.
</details>

<details>
  <summary>I registered a new upgrade, but only one node is 'up to date'?</summary>

Remember that Blazar refreshes its internal state periodically. If you registered a new upgrade on one instance with the 'DATABASE' provider and the other node doesn't see it, you have two options:
1. Wait for Blazar to sync (see 'Time to next sync' in the UI).
2. Force sync via UI/CLI/RPC call.
</details>

<details>
  <summary>Does Blazar work with chains with non-standard gov module (e.g., Neutron)?</summary>

Yes, but you'll need to register manually a `GOVERNANCE` type upgrade in `LOCAL` or `DATABASE` provider.

Neutron is a smart contract chain that implements its own governance (DAO DAO) via an on-chain contract. Blazar currently doesn't understand the custom smart contract logic, therefore the operator cannot use the `CHAIN` provider.
However, the Neutron governance is integrated with Cosmos SDK upgrades module and will output the `upgrade-info.json` at the upgrade height. Therefore from Blazar perspective, the `GOVERNANCE` type is valid, but the source provide must be different.
</details>

<details>
  <summary>What is the difference between 'compose-file' and 'env-file' upgrade mode?</summary>

When performing a node upgrade, Blazar updates the docker version tag (e.g., `v1.0.0` to `v2.0.0`). That version is stored in the `docker-compose.yaml` file in the following form:
```
$ cat docker-compose.yaml | grep 'image'
image: <client_id>.dkr.ecr.us-east-1.amazonaws.com/chorusone/archway:v1.0.0
```

or in the `.env` file:
```
$ cat docker-compose.yaml | grep 'image'
image: <client_id>.dkr.ecr.us-east-1.amazonaws.com/chorusone/archway:${VERSION_archway}

$ cat .env
VERSION_archway=1.0.0
```

> Why do we support both upgrade modes and which one is better?

The `compose-file` is simpler, but we highly recommend the `env-file` mode. If the version tag is stored in the `.env` file, the blast radius of possible mistakes is very low, unlike editing the whole `docker-compose.yaml` to replace one single variable.
</details>

<details>
  <summary>What is the purpose of SQL migration files?</summary>

This question is relevant for anyone who wants to use the DATABASE provider.

Blazar leverages [GORM](https://gorm.io) to manage SQL databases. If you enable automatic migrations by setting:
```toml
[upgrade-registry.provider.database]
auto-migrate = true
```

you can disregard the `migrations` files since [GORM](https://gorm.io) will automatically initialize all necessary SQL tables.

However, if `auto-migrate` is disabled, you'll need to manually apply the migration SQL statements.
</details>

## License
Blazar is licensed under the Apache 2.0 License. For more detailed information, please refer to the LICENSE file in the repository.
