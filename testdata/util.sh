#!/bin/sh
case "$1" in
  success) printf "ok"; exit 0 ;;
  fail)    printf "boom" >&2; exit 3 ;;
  print)   cat; exit 0 ;;
  pwd)     pwd; exit 0 ;;
  *)       exit 2 ;;
esac
