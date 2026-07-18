#! /bin/bash
rm -rf linuxbin
rm -rf osxbin
rm -rf winbin

mkdir linuxbin
mkdir osxbin
mkdir winbin

bash -v build.sh goterm
