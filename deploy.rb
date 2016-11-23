#!/usr/bin/env ruby
# frozen_string_literal: true
#
# Copyright (C) 2016 Harald Sitter <sitter@kde.org>
#
# This program is free software; you can redistribute it and/or
# modify it under the terms of the GNU General Public License as
# published by the Free Software Foundation; either version 3 of
# the License or any later version accepted by the membership of
# KDE e.V. (or its successor approved by the membership of KDE
# e.V.), which shall act as a proxy defined in Section 14 of
# version 3 of the license.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.

require 'net/ssh'

bin = 'neon-contents-grapple'
user = 'neonapis'
host = 'darwini.kde.org'
path = "/home/#{user}/bin/"

system('rsync', '-avz', '--progress',
       '-e', 'ssh', bin, "#{user}@#{host}:#{path}") || raise

system('ssh', "#{user}@#{host}",
       'systemctl --user restart neon-contents-grapple.service') || raise
