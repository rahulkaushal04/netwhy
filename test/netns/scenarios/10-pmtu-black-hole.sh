DESC="router MTU 1400 and ICMP fragmentation-needed dropped"
EXPECT_WARN=pmtu
break_it() {
  ip -n nw-router link set r1 mtu 1400
  ip netns exec nw-router nft add table inet f
  ip netns exec nw-router nft add chain inet f out "{ type filter hook output priority 0; }"
  ip netns exec nw-router nft add rule inet f out icmp type destination-unreachable icmp code frag-needed drop
}
