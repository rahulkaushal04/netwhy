DESC="nftables drops port 443"
EXPECT_LAYER=tcp
EXPECT_TEXT="timed out"
break_it() {
  ip netns exec nw-server nft add table inet f
  ip netns exec nw-server nft add chain inet f in "{ type filter hook input priority 0; }"
  ip netns exec nw-server nft add rule inet f in tcp dport 443 drop
}
