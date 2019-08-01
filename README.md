# securelink [![securelink](https://godoc.org/github.com/alexandrestein/securelink?status.svg)](https://godoc.org/github.com/alexandrestein/securelink) [![codebeat badge](https://codebeat.co/badges/bdcbeb55-fa63-4a0f-93b6-14561f22a37d)](https://codebeat.co/projects/github-com-alexandrestein-securelink-master) [![Build Status](https://travis-ci.org/alexandrestein/securelink.svg?branch=master)](https://travis-ci.org/alexandrestein/securelink) [![Coverage Status](https://coveralls.io/repos/github/alexandrestein/securelink/badge.svg)](https://coveralls.io/github/alexandrestein/securelink) [![Go Report Card](https://goreportcard.com/badge/github.com/alexandrestein/securelink)](https://goreportcard.com/report/github.com/alexandrestein/securelink) [![License](https://img.shields.io/github/license/alexandrestein/securelink.svg)](https://www.apache.org/licenses/LICENSE-2.0)

**Securelink** tries to provides an easy to use and efficient way to use multiple interconnected services without the need to open and to manage many open ports.

The idea is to have a secure tunnel to make all services use only one channel.

A basic use case can be an clustered service which is a RAFT protocol for consensus. The services will probably need to speak to other nodes but you can't use RAFT for this.
In usual case you will need to open at least two ports on all your nodes. Maybe every connection use a different way to secure them selfs.

In this example we speak only about two service but you can have many more.

This is the purpose of the package.

## status

The project is not ready and is under development.