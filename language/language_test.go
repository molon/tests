package language

import (
	"testing"

	"golang.org/x/text/language"
)

func TestMatchStrings(t *testing.T) {
	tags := []language.Tag{
		language.SimplifiedChinese, // zh-Hans
		language.English,           // en
	}
	matcher := language.NewMatcher(tags)
	tag, index := language.MatchStrings(matcher, "en-US")
	// output: tag: en-u-rg-uszzzz, index: 1
	// 注意这里返回的不是 tag: en, index: 1
	t.Logf("tag: %v, index: %v", tag, index)

	// 所以结论是，MatchStrings 返回的 tag 是最匹配的 tag，而不是最匹配的 base tag，这个要注意，所以我们取用的时候应该通过 index 和 tags[index] 来取
	// 这样才好和最终的 i18n 配置文件对应上
	t.Logf("tag: %v, index: %v", tags[index], index)
}
