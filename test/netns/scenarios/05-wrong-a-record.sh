DESC="dnsmasq returns the router, which has nothing on 443"
DNS_RULES="/$NAME/10.0.1.1"
EXPECT_LAYER=tcp
EXPECT_TEXT="refused"
