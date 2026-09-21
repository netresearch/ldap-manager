package web

// unreachableLDAPServer is the server address for test clients that must
// never reach a directory.
//
// simple-ldap-go up to v1.17.0 recognised fixture host names such as
// "test.server.com" and returned a client that never dialled. From v1.18.0
// the host name decides nothing (netresearch/simple-ldap-go#246): New dials
// unless Config.SkipConnectionCheck is set, and every later directory call
// dials regardless. A fixture host name therefore costs a real DNS lookup,
// which took up to 540 ms here and pushed fiber.App.Test past its 1 s default
// under -race.
//
// Loopback port 1 has no listener, so the kernel refuses the connection at
// once and the test does not depend on the machine's resolver.
const unreachableLDAPServer = "ldap://127.0.0.1:1"
