# securelink

**Securelink** tries to provides an easy to use and efficient way to use multiple interconnected services without the need to open and to manage many open ports.

The idea is to have a secure tunnel to make all services use only one channel.

A basic use case can be an clustered service which is a RAFT protocol for consensus. The services will probably need to speak to other nodes but you can't use RAFT for this.
In usual case you will need to open at least two ports on all your nodes. Maybe every connection use a different way to secure them selfs.

In this example we speak only about two service but you can have many more.

This is the purpose of the package.