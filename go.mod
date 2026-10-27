module github.com/sebamedina/gpg-cracker

go 1.24.7

require github.com/ProtonMail/go-crypto v1.1.6

require (
	github.com/cloudflare/circl v1.3.7 // indirect
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
)

replace golang.org/x/crypto => github.com/golang/crypto v0.17.0

replace golang.org/x/sys => github.com/golang/sys v0.16.0
