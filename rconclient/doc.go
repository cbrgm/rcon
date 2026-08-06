// Package rconclient is a high-level RCON client built on
// github.com/cbrgm/rcon.
//
// It mirrors the shape of net/http: a [DefaultClient], package-level helper
// functions that delegate to it, and an instantiable [Client] that is safe for
// concurrent use. For repeated commands to one server, open a [Session].
//
// Most servers work with the default multi-packet response mode. For servers
// that mishandle the multi-packet terminator sentinel, use [WithSinglePacket];
// for servers that also split large replies across packets (such as Project
// Zomboid), use [WithReadUntilIdle].
package rconclient
