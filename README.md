# Building

- set GOPATH in envrionment
- install go
- go get -u github.com/mattn/gom # dependency helper
- gom -production install # install dependencies
- gom build # build
- ./neon-contents-grapple

# Installation

- adjust systemd/* as you need it
- copy systemd/* to ~/.config/systemd/user/ or a system location depending on
  whether you want to run it as system service or user service
- when using a user service make sure to enable login lingering for that
  user with `loginctl enable-linger $username`
- enable the systemd service `systemctl --user enable neon-contents-grapple.service`
  (or without --user for system service)
- start it `systemctl --user start neon-contents-grapple.service`
- make sure it is running `systemctl --user status neon-contents-grapple.service`
- it should now persist through logouts and due to linking into default target is should autostart on reboots

## notes

- in PWD my.db is created for the database, so make sure you are in a suitable PWD
- binary should be run through systemd
