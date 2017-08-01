all:

go-test:
	@find * -name '*_test.go' |\
	sed -e 's@^@github.com/Symantec/tricorder/@' -e 's@/[^/]*$$@@' |\
	sort -u | xargs go test
