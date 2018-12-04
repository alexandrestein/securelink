#!/bin/bash
shopt -s globstar
rootdir=`pwd`

CI_NAME="travis"

# go test -race -v -cover -short -coverprofile=cov_part.out
go test -race -v -cover -coverprofile=cov_part.out

for dir in **/ ; do
  cd $dir
  exec 5>&1
  out=$(go test -race -v -coverprofile=cov_part.out | tee >(cat - >&5))
  if [ $? -eq 1 ] ; then
    if [ $(cat $out | grep -o 'no buildable Go source files') == "" ] ; then
      echo "Tests failed! Exiting..." ; exit 1
    fi
  fi
  cd $rootdir
done

if [ -z "$CI_NAME" ]; then
  echo "CI_NAME is unset. Skipping coverage report!"
  exit 0
fi

find . -name cov_part.out | xargs cat > cov.out
# make sure we do not run the ruby gem which is first in $PATH :(
codeclimate-test-reporter < cov.out