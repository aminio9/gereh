package application

import (
	"bytes"
	"fmt"
	"unicode/utf8"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
)

const (
	minCredentialBytes = 8
	maxCredentialBytes = 8192
)

func normalizeCredential(
	value string,
) ([]byte, error) {
	raw :=
		bytes.TrimSpace(
			[]byte(value),
		)

	if len(raw) <
		minCredentialBytes ||
		len(raw) >
			maxCredentialBytes {
		return nil,
			fmt.Errorf(
				"%w: credential length is invalid",
				domain.ErrInvalidArgument,
			)
	}

	if !utf8.Valid(raw) {
		return nil,
			fmt.Errorf(
				"%w: credential is not valid UTF-8",
				domain.ErrInvalidArgument,
			)
	}

	if bytes.IndexByte(
		raw,
		0,
	) >= 0 ||
		bytes.IndexByte(
			raw,
			'\r',
		) >= 0 ||
		bytes.IndexByte(
			raw,
			'\n',
		) >= 0 {
		return nil,
			fmt.Errorf(
				"%w: credential contains invalid characters",
				domain.ErrInvalidArgument,
			)
	}

	result :=
		make(
			[]byte,
			len(raw),
		)

	copy(
		result,
		raw,
	)

	return result, nil
}

func zeroBytes(
	value []byte,
) {
	for index := range value {
		value[index] = 0
	}
}
