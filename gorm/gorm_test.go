package gorm

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRowsAffected(t *testing.T) {
	require.NoError(t, db.AutoMigrate(&User{}))

	users := []*User{
		{Name: "Alice"},
		{Name: "Bob"},
	}
	require.NoError(t, db.Create(users).Error)
	firstUserID := users[0].ID

	firstUser := &User{}
	result := db.Where("id = ?", firstUserID).First(&firstUser)
	require.NoError(t, result.Error)
	require.Equal(t, int64(1), result.RowsAffected) // 查到一条数据，此处会返回 1

	users = []*User{}
	result = db.Find(&users)
	require.NoError(t, result.Error)
	require.Equal(t, int64(2), result.RowsAffected) // 查到两条数据，此处会返回 2

	users = []*User{}
	result = db.First(&users) // First 到一个数组里，最终 sql 语句还是 LIMIT 1
	require.NoError(t, result.Error)
	require.Equal(t, int64(1), result.RowsAffected) // 所以此处还是返回 1

	users = []*User{} // First 到一个数组里，该返回 ErrRecordNotFound 还是会返回 ErrRecordNotFound
	require.ErrorIs(t, db.First(&users, "name = ?", "not found").Error, gorm.ErrRecordNotFound)

	require.NoError(t, db.Exec("TRUNCATE TABLE users").Error)
}

func TestUpdate(t *testing.T) {
	require.NoError(t, db.AutoMigrate(&User{}))

	users := []*User{
		{Name: "Alice"},
		{Name: "Bob"},
	}
	require.NoError(t, db.Create(users).Error)

	oldUpdatedAt := users[0].UpdatedAt
	result := db.Model(users[0]).Where("name = ?", "Alice").Update("name", "Alice 2")
	require.NoError(t, result.Error)
	require.Equal(t, int64(1), result.RowsAffected)
	require.NotEqual(t, oldUpdatedAt, users[0].UpdatedAt) // RowsAffected 为 1 的情况下会更新 UpdatedAt

	oldUpdatedAt = users[0].UpdatedAt
	result = db.Model(users[0]).Where("name = ? OR name = ?", "Alice 2", "Bob").Update("name", "David")
	require.NoError(t, result.Error)
	require.Equal(t, int64(1), result.RowsAffected) // 由于 db.Model 给到的 struct 是有主键的，所以只会更新第一条记录
	require.NotEqual(t, oldUpdatedAt, users[0].UpdatedAt)

	userModel := &User{}
	result = db.Model(userModel).Where("name = ? OR name = ?", "David", "Bob").Update("name", "Eve")
	require.NoError(t, result.Error)
	require.Equal(t, int64(2), result.RowsAffected)
	require.NotZero(t, userModel.UpdatedAt) // RowsAffected 为 2 的情况下也会更新 UpdatedAt
	require.Equal(t, "Eve", userModel.Name) // 会更新已指定更新的对应字段
	require.Zero(t, userModel.ID)           // 不会更新未指定更新的字段
	t.Logf("userModel: %v", userModel)

	firstUser := users[0]                                                              // 此时的 Name 为 David ，数据库中的 Name 为 Eve
	result = db.Model(firstUser).Where("id = ?", firstUser.ID).Update("name", "Frank") // 因为 firstUser 的主键有值 sql 会有两个 id 的条件
	require.NoError(t, result.Error)
	require.Equal(t, int64(1), result.RowsAffected)
	require.Equal(t, "Frank", firstUser.Name) // 会更新 struct 中的对应字段

	require.NoError(t, db.Model(&User{}).Where("name = ?", "Frank").Update("age", 18).Error) // 会更新 firstUser 记录的 age 为 18
	// 此时 firstUser 的 Age 为 0 ，数据库中的 Age 为 18
	result = db.Model(firstUser).Update("name", "Mike")
	require.NoError(t, result.Error)
	require.Equal(t, int64(1), result.RowsAffected)
	require.Equal(t, "Mike", firstUser.Name) // 会更新 struct 中的对应字段
	require.Equal(t, 0, firstUser.Age)       // 不会更新 Update 时没有指定的字段为数据库记录的值

	firstUserID := firstUser.ID
	{
		firstUser := &User{}
		result := db.Model(firstUser).Where("id = ?", firstUserID).Update("name", "MikeX")
		require.NoError(t, result.Error)
		require.Equal(t, int64(1), result.RowsAffected) // 更新了一条记录
		require.Zero(t, firstUser.ID)                   // 即使只更新了一条记录，此时也不会设置主键
		t.Logf("firstUser: %v", firstUser)
		result = result.Order("id DESC").First(&firstUser) // 从下面可以看出，此时就不会是依赖于 firstUser 里的空 ID 来获取了，最终 sql 里的 id = 1 这个条件是从前一个行为中获取的
		require.NoError(t, result.Error)
		require.Equal(t, int64(1), result.RowsAffected) // 还是会返回 1
		require.NotZero(t, firstUser.ID)                // 会查询到所有信息
		t.Logf("firstUser: %v", firstUser)
	}

	require.NoError(t, db.Exec("TRUNCATE TABLE users").Error)
}
