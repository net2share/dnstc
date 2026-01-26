# DNS Tunnel Client (dnstc)

A cross-platform client tool for connecting to DNS tunnel servers from restricted networks. Supports Windows, macOS, and Linux.

## Features

- Download and configure Slipstream and DNSTT in standalone mode
- Install Slipstream as a Shadowsocks SIP003 plugin with Shadowsocks client
- Discover working recursive DNS resolver IPs using dnst-resolver-scanner
- Continuously monitor resolver health and maintain a pool of working resolvers
- Run a local DNS proxy with load balancing across multiple resolvers
- Run multiple transport instances with load balancing for higher aggregate bandwidth
- Orchestrate the entire flow between scanner, DNS proxy, and transports

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              dnstc                                          │
│                                                                             │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────────┐  │
│  │   Orchestrator  │───►│ Resolver Scanner│───►│ ir-resolvers (raw IPs)  │  │
│  └────────┬────────┘    └────────┬────────┘    └─────────────────────────┘  │
│           │                      │                                          │
│           │                      ▼                                          │
│           │             ┌─────────────────┐                                 │
│           │             │   DNS Proxy     │                                 │
│           │             │  (gost)         │                                 │
│           │             │                 │                                 │
│           │             │ • Load balancer │                                 │
│           │             │ • Health monitor│                                 │
│           │             │ • Resolver pool │                                 │
│           │             └────────┬────────┘                                 │
│           │                      │                                          │
│           ▼                      ▼                                          │
│  ┌─────────────────────────────────────────────────────────────┐            │
│  │              Transport Load Balancer (gost)                 │            │
│  │         Higher aggregate bandwidth via multiple instances   │            │
│  └──────┬──────────────────┬──────────────────┬───────────────┘            │
│         │                  │                  │                             │
│         ▼                  ▼                  ▼                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                      │
│  │ Slipstream  │    │ Slipstream  │    │   DNSTT     │                      │
│  │ Instance 1  │    │ Instance 2  │    │  Instance   │                      │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘                      │
│         │                  │                  │                             │
└─────────┼──────────────────┼──────────────────┼─────────────────────────────┘
          │                  │                  │
          └──────────────────┼──────────────────┘
                             │
                             ▼ DNS Queries
                    ┌─────────────────┐
                    │ Resolver Pool   │
                    │ (Iran Intranet) │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │  DNS Tunnel     │
                    │  Server (dnstm) │
                    └─────────────────┘
```

## Components

### Resolver Scanner Integration

Uses [dnst-resolver-scanner](https://github.com/net2share/dnst-resolver-scanner) to:
1. Scan raw resolver list from [ir-resolvers](https://github.com/net2share/ir-resolvers)
2. Find working recursive resolvers
3. Validate with actual tunnel protocols (e2e test via server health check endpoints)

### DNS Proxy

Local DNS proxy using [gost](https://github.com/go-gost/gost):
- Load balances across multiple healthy resolvers
- Continuously monitors resolver health
- Removes failed resolvers from pool
- Uses gost's [Selector](https://gost.run/en/concepts/selector/) for load balancing strategies

### Transport Load Balancer

Multiple transport instances with load balancing for higher bandwidth:
- Run multiple Slipstream/DNSTT instances concurrently
- Distribute connections across instances using gost
- Aggregate bandwidth from multiple tunnels
- Automatic failover when instances fail

### Transport Modes

- **Standalone**: Direct Slipstream or DNSTT client
- **SIP003 Plugin**: Slipstream as Shadowsocks plugin with shadowsocks-rust client

## Requirements

- Windows, macOS, or Linux
- Network access to recursive DNS resolvers (Iran intranet for target use case)

## Related Projects

- [dnstm](https://github.com/net2share/dnstm) - Server-side DNS tunnel manager
- [dnst-resolver-scanner](https://github.com/net2share/dnst-resolver-scanner) - Resolver scanner
- [ir-resolvers](https://github.com/net2share/ir-resolvers) - Iran resolver IP list
- [go-corelib](https://github.com/net2share/go-corelib) - Shared Go library
