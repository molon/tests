package json

import (
	"testing"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/stretchr/testify/assert"
)

func TestJSONMergePatchNullValueRFC7386(t *testing.T) {
	original := `{"a": 1, "b": 2, "c": null}`
	patch := `{"a": null, "d": 3, "e": null}`

	{
		// JSON Merge Patch: https://datatracker.ietf.org/doc/html/rfc7386
		// patch 里的 null 值表示删除原字段
		merged, err := jsonpatch.MergePatch([]byte(original), []byte(patch))
		assert.NoError(t, err)
		assert.JSONEq(t, `{"b": 2, "c": null, "d": 3}`, string(merged))
	}

	{
		// 不符合 rfc7386 标准，但是对于 struct 来说，它会更好用，通常 struct 没有删除字段一说
		// patch 里的 null 值表示添加字段或者覆盖原字段
		merged, err := jsonpatch.MergeMergePatches([]byte(original), []byte(patch))
		assert.NoError(t, err)
		assert.JSONEq(t, `{"a": null, "b": 2, "c": null, "d": 3, "e": null}`, string(merged))
	}
}
