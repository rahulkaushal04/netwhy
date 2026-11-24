DESC="resolv.conf points at a dead IP"
EXPECT_LAYER=dns
EXPECT_TEXT="no DNS resolver answered"
break_it() { echo "nameserver 10.0.1.99" >/etc/netns/nw-client/resolv.conf; }
