DESC="nftables rejects port 443 with a reset"
EXPECT_LAYER=tcp
EXPECT_TEXT="refused"
break_it() {
  ip netns exec nw-server nft add table inet f
  ip netns exec nw-server nft add chain inet f in "{ type filter hook input priority 0; }"
  ip netns exec nw-server nft add rule inet f in tcp dport 443 reject with tcp reset
}
