DESC="default route deleted"
EXPECT_LAYER=system
break_it() { ip -n nw-client route del default; }
