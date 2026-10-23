//go:build !speech || !linux

package speech

import "errors"

func NewAzureProvider(key, region string) (Provider, error) {
	return nil, errors.New("Build the Linux speech container to enable the native Azure SDK.")
}
