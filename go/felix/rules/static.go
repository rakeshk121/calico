// Copyright (c) 2016-2017 Tigera, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rules

import (
	log "github.com/Sirupsen/logrus"
	. "github.com/projectcalico/felix/go/felix/iptables"
	"strings"
)

func (r *ruleRenderer) StaticFilterTableChains(ipVersion uint8) (chains []*Chain) {
	chains = append(chains, r.StaticFilterForwardChains()...)
	chains = append(chains, r.StaticFilterInputChains(ipVersion)...)
	chains = append(chains, r.StaticFilterOutputChains()...)
	return
}

const (
	ProtoIPIP   = 4
	ProtoICMPv6 = 58
)

func (r *ruleRenderer) StaticFilterInputChains(ipVersion uint8) []*Chain {
	return []*Chain{
		r.filterInputChain(ipVersion),
		r.filterWorkloadToHostChain(ipVersion),
		r.filterFailsafeInChain(),
	}
}

func (r *ruleRenderer) filterInputChain(ipVersion uint8) *Chain {
	var inputRules []Rule

	if ipVersion == 4 && r.IPIPEnabled {
		// IPIP is enabled, filter incoming IPIP packets to ensure they come from a
		// recognised host.  We use the protocol number rather than its name because the
		// name is not guaranteed to be known by the kernel.
		match := Match().ProtocolNum(ProtoIPIP).
			NotSourceIPSet(r.IPSetConfigV4.NameForMainIPSet(IPSetIDAllHostIPs))
		inputRules = append(inputRules,
			r.DropRules(match, "Drop IPIP packets from non-Calico hosts")...)
	}

	// Allow established connections via the conntrack table.
	inputRules = append(inputRules, r.DropRules(Match().ConntrackState("INVALID"))...)
	inputRules = append(inputRules,
		Rule{
			Match:  Match().ConntrackState("RELATED,ESTABLISHED"),
			Action: AcceptAction{},
		},
	)

	// Apply our policy to packets coming from workload endpoints.
	for _, prefix := range r.WorkloadIfacePrefixes {
		log.WithField("ifacePrefix", prefix).Debug("Adding workload match rules")
		ifaceMatch := prefix + "+"
		inputRules = append(inputRules, Rule{
			Match:  Match().InInterface(ifaceMatch),
			Action: GotoAction{Target: ChainWorkloadToHost},
		})
	}

	// Apply host endpoint policy.
	inputRules = append(inputRules, Rule{
		Action: GotoAction{Target: ChainDispatchFromHostEndpoint},
	})

	return &Chain{
		Name:  ChainFilterInput,
		Rules: inputRules,
	}
}

func (r *ruleRenderer) filterWorkloadToHostChain(ipVersion uint8) *Chain {
	var rules []Rule

	// For IPv6, we need to white-list certain ICMP traffic from workloads in order to to act
	// as a router.  Note: we do this before the policy chains, so we're bypassing the egress
	// rules for this traffic.  While that might be unexpected, it makes sure that the user
	// doesn't cut off their own connectivity in subtle ways that they shouldn't have to worry
	// about.
	//
	// - 130: multicast listener query.
	// - 131: multicast listener report.
	// - 132: multicast listener done.
	// - 133: router solicitation, which an endpoint uses to request
	//        configuration information rather than waiting for an
	//        unsolicited router advertisement.
	// - 135: neighbor solicitation.
	// - 136: neighbor advertisement.
	if ipVersion == 6 {
		for _, icmpType := range []uint8{130, 131, 132, 133, 135, 136} {
			rules = append(rules, Rule{
				Match: Match().
					ProtocolNum(ProtoICMPv6).
					ICMPV6Type(icmpType),
				Action: AcceptAction{},
			})
		}
	}

	if r.OpenStackSpecialCasesEnabled {
		if ipVersion == 4 && r.OpenStackMetadataIP != nil {
			// For OpenStack compatibility, we support a special-case to allow incoming traffic
			// to the OpenStack metadata IP/port.
			// TODO(smc) Long-term, it'd be nice if the OpenStack plugin programmed a policy to
			// do this instead.
			log.WithField("ip", r.OpenStackMetadataIP).Info(
				"OpenStack metadata IP specified, installing whitelist rule.")
			rules = append(rules, Rule{
				Match: Match().
					Protocol("tcp").
					DestNet(r.OpenStackMetadataIP.String()).
					DestPorts(r.OpenStackMetadataPort),
				Action: AcceptAction{},
			})
		}

		// Again, for OpenStack compatibility, white-list certain protocols.
		// TODO(smc) Long-term, it'd be nice if the OpenStack plugin programmed a policy to
		// do this instead.
		dhcpSrcPort := uint16(68)
		dhcpDestPort := uint16(67)
		if ipVersion == 6 {
			dhcpSrcPort = uint16(546)
			dhcpDestPort = uint16(547)
		}
		dnsDestPort := uint16(53)
		rules = append(rules,
			Rule{
				Match: Match().
					Protocol("udp").
					SourcePorts(dhcpSrcPort).
					DestPorts(dhcpDestPort),
				Action: AcceptAction{},
			},
			Rule{
				Match: Match().
					Protocol("udp").
					DestPorts(dnsDestPort),
				Action: AcceptAction{},
			},
		)
	}

	// Now send traffic to the policy chains to apply the egress policy.
	rules = append(rules, Rule{
		Action: JumpAction{Target: ChainFromWorkloadDispatch},
	})

	// If the dispatch chain accepts the packet, it returns to us here.  Apply the configured
	// action.  Note: we may have done work above to allow the packet and then end up dropping
	// it here.  We can't optimize that away because there may be other rules (such as log
	// rules in the policy).
	for _, action := range r.inputAcceptActions {
		rules = append(rules, Rule{
			Action:  action,
			Comment: "Configured DefaultEndpointToHostAction",
		})
	}

	return &Chain{
		Name:  ChainWorkloadToHost,
		Rules: rules,
	}
}

func (r *ruleRenderer) filterFailsafeInChain() *Chain {
	rules := []Rule{}

	for _, port := range r.Config.FailsafeInboundHostPorts {
		rules = append(rules, Rule{
			Match:  Match().Protocol("tcp").DestPorts(port),
			Action: AcceptAction{},
		})
	}

	return &Chain{
		Name:  ChainFailsafeIn,
		Rules: rules,
	}
}

func (r *ruleRenderer) filterFailsafeOutChain() *Chain {
	rules := []Rule{}

	for _, port := range r.Config.FailsafeOutboundHostPorts {
		rules = append(rules, Rule{
			Match:  Match().Protocol("tcp").DestPorts(port),
			Action: AcceptAction{},
		})
	}

	return &Chain{
		Name:  ChainFailsafeOut,
		Rules: rules,
	}
}

func (r *ruleRenderer) StaticFilterForwardChains() []*Chain {
	rules := []Rule{}

	// conntrack rules to accept established connections.
	rules = append(rules, r.DropRules(Match().ConntrackState("INVALID"))...)
	rules = append(rules,
		Rule{
			Match:  Match().ConntrackState("RELATED,ESTABLISHED"),
			Action: AcceptAction{},
		},
	)

	// To handle multiple workload interface prefixes, we want 2 batches of rules.
	//
	// The first dispatches the packet to our dispatch chains if it is going to/from an
	// interface that we're responsible for.  Note: the dispatch chains represent "allow" by
	// returning to this chain for further processing; this is required to handle traffic that
	// is going between endpoints on the same host.  In that case we need to apply the egress
	// policy for one endpoint and the ingress policy for the other.
	//
	// The second batch actually accepts the packets if they passed through the workload policy
	// and were returned.

	// Jump to dispatch chains.
	for _, prefix := range r.WorkloadIfacePrefixes {
		log.WithField("ifacePrefix", prefix).Debug("Adding workload match rules")
		ifaceMatch := prefix + "+"
		rules = append(rules,
			Rule{
				Match:  Match().InInterface(ifaceMatch),
				Action: JumpAction{Target: ChainFromWorkloadDispatch},
			},
			Rule{
				Match:  Match().OutInterface(ifaceMatch),
				Action: JumpAction{Target: ChainToWorkloadDispatch},
			},
		)
	}

	// Accept if everything above passed.
	for _, prefix := range r.WorkloadIfacePrefixes {
		log.WithField("ifacePrefix", prefix).Debug("Adding workload match rules")
		ifaceMatch := prefix + "+"
		rules = append(rules,
			Rule{
				Match:  Match().InInterface(ifaceMatch),
				Action: AcceptAction{},
			},
			Rule{
				Match:  Match().OutInterface(ifaceMatch),
				Action: AcceptAction{},
			},
		)
	}

	// If we get here, the packet is not going to or from a workload, but, since we're in the
	// FORWARD chain, it is being forwarded.  Apply host endpoint rules in that case.  This
	// allows Calico to police traffic that is flowing through a NAT gateway or router.
	rules = append(rules,
		Rule{
			Action: JumpAction{Target: ChainDispatchFromHostEndpoint},
		},
		Rule{
			Action: JumpAction{Target: ChainDispatchToHostEndpoint},
		},
	)

	return []*Chain{{
		Name:  ChainFilterForward,
		Rules: rules,
	}}
}

func (r *ruleRenderer) StaticFilterOutputChains() []*Chain {
	return []*Chain{
		r.filterOutputChain(),
		r.filterFailsafeOutChain(),
	}
}

func (r *ruleRenderer) filterOutputChain() *Chain {
	rules := []Rule{}

	// conntrack rules.
	rules = append(rules, r.DropRules(Match().ConntrackState("INVALID"))...)
	rules = append(rules,
		Rule{
			Match:  Match().ConntrackState("RELATED,ESTABLISHED"),
			Action: AcceptAction{},
		},
	)

	// We don't currently police host -> endpoint according to the endpoint's ingress policy.
	// That decision is based on pragmatism; it's generally very useful to be able to contact
	// any local workload from the host and policing the traffic doesn't really protect
	// against host compromise.  If a host is compromised, then the rules could be removed!
	for _, prefix := range r.WorkloadIfacePrefixes {
		// If the packet is going to a worklaod endpoint, RETURN.
		log.WithField("ifacePrefix", prefix).Debug("Adding workload match rules")
		ifaceMatch := prefix + "+"
		rules = append(rules,
			Rule{
				Match:  Match().OutInterface(ifaceMatch),
				Action: ReturnAction{},
			},
		)
	}

	// If we reach here, the packet is not going to a workload so it must be going to a
	// host endpoint.

	// Apply host endpoint policy.
	rules = append(rules, Rule{
		Action: GotoAction{Target: ChainDispatchToHostEndpoint},
	})

	return &Chain{
		Name:  ChainFilterOutput,
		Rules: rules,
	}
}

func (r *ruleRenderer) StaticNATTableChains(ipVersion uint8) (chains []*Chain) {
	chains = append(chains, r.StaticNATPreroutingChains(ipVersion)...)
	chains = append(chains, r.StaticNATPostroutingChains(ipVersion)...)
	return
}

func (r *ruleRenderer) StaticNATPreroutingChains(ipVersion uint8) []*Chain {
	rules := []Rule{}

	if ipVersion == 4 && r.OpenStackMetadataIP != nil {
		rules = append(rules, Rule{
			Match: Match().
				Protocol("tcp").
				DestPorts(80).
				DestNet("169.254.169.254/32"),
			Action: DNATAction{
				DestAddr: r.OpenStackMetadataIP.String(),
				DestPort: r.OpenStackMetadataPort,
			},
		})
	}

	return []*Chain{{
		Name:  ChainNATPrerouting,
		Rules: rules,
	}}
}

func (r *ruleRenderer) StaticNATPostroutingChains(ipVersion uint8) []*Chain {
	rules := []Rule{
		{
			Action: JumpAction{Target: ChainNATOutgoing},
		},
	}
	if r.IPIPEnabled && len(r.IPIPTunnelAddress) > 0 {
		// Add a rule to catch packets that are being sent down the IPIP tunnel from an
		// incorrect local IP address of the host and NAT them to use the tunnel IP as its
		// source.  This happens if:
		//
		// - the user explicitly binds their socket to the wrong source IP accidentally
		// - the user sends traffic to, for example, a Kubernetes service IP, which is
		//   implemented via NAT instead of routing, leading the kernel to choose the
		//   wrong source IP.
		//
		// We NAT the source of the packet to use the tunnel IP.  We assume that
		// non-local IPs have been correctly routed.  Since Calico-assigned IPs are
		// non-local (because they're down a veth), they won't get caught by the rule.
		// Other remote sources will only reach the tunnel if they're being NATted
		// already (for example, a Kubernetes "NodePort").  The kernel will then
		// choose the correct source on its own.
		rules = append(rules, Rule{
			Match: Match().
				// Only match packets going out the tunnel.
				OutInterface("tunl0").
				// Match packets that don't have the correct source address.  This
				// matches local addresses (i.e. ones assigned to this host)
				// limiting the match to the output interface (which we matched
				// above as the tunnel).  Avoiding embedding the IP address lets
				// us use a static rule, which is easier to manage.
				NotSrcAddrType(AddrTypeLocal, true).
				// Only match if the IP is also some local IP on the box.  This
				// prevents us from matching packets from workloads, which are
				// remote as far as the routing table is concerned.
				SrcAddrType(AddrTypeLocal, false),
			Action: MasqAction{},
		})
	}
	return []*Chain{{
		Name:  ChainNATPostrouting,
		Rules: rules,
	}}
}

func (r ruleRenderer) DropRules(matchCriteria MatchCriteria, comments ...string) []Rule {
	rules := []Rule{}

	for _, action := range r.DropActions() {
		rules = append(rules, Rule{
			Match:   matchCriteria,
			Action:  action,
			Comment: strings.Join(comments, "; "),
		})
	}

	return rules
}

func (r *ruleRenderer) DropActions() []Action {
	return r.dropActions
}
