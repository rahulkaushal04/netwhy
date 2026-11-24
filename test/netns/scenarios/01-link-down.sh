DESC="client link down"
EXPECT_LAYER=system
break_it() { ip -n nw-client link set c0 down; }
