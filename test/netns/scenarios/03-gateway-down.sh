DESC="router does not answer ARP"
EXPECT_LAYER=gateway
break_it() { ip -n nw-router link set r0 down; }
