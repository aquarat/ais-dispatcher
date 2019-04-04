# AIS Dispatcher
AIS Dispatcher is a simple application that takes AIS data from a serial port and streams it out via UDP to a remote server. It optionally writes the received packets to a SQLite database. It is used to stream data out to MarineTraffic.com using a Raspberry Pi

### Building
go get -v github.com/aquarat/ais-dispatcher

