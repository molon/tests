package gorm

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type UserWithHook struct {
	gorm.Model
	Name string
	logs []string
}

func (u *UserWithHook) cleanLogs() {
	u.logs = []string{}
}

func (u *UserWithHook) BeforeSave(tx *gorm.DB) error {
	u.logs = append(u.logs, "BeforeSave")
	return nil
}

func (u *UserWithHook) BeforeCreate(tx *gorm.DB) error {
	u.logs = append(u.logs, "BeforeCreate")
	return nil
}

func (u *UserWithHook) BeforeUpdate(tx *gorm.DB) (err error) {
	u.logs = append(u.logs, "BeforeUpdate")
	return nil
}

func (u *UserWithHook) BeforeDelete(tx *gorm.DB) (err error) {
	u.logs = append(u.logs, "BeforeDelete")
	return nil
}

func TestHooks(t *testing.T) {
	require.NoError(t, db.AutoMigrate(&UserWithHook{}))

	{
		user := UserWithHook{
			Name: "Alice",
		}
		// 执行 Create 时，也会执行 BeforeSave
		require.NoError(t, db.Create(&user).Error)
		require.Len(t, user.logs, 2)
		require.Equal(t, "BeforeSave", user.logs[0])
		require.Equal(t, "BeforeCreate", user.logs[1])

		require.NoError(t, db.Exec("TRUNCATE TABLE user_with_hooks").Error)
	}

	{
		user := UserWithHook{
			Name: "Alice",
		}
		// 执行 Save 时，也会执行 BeforeCreate ，和 Create 没差
		require.NoError(t, db.Save(&user).Error)
		require.Len(t, user.logs, 2)
		require.Equal(t, "BeforeSave", user.logs[0])
		require.Equal(t, "BeforeCreate", user.logs[1])

		user.cleanLogs()
		user.Name = "Bob"
		// 如果发现 Save 最终执行的是 Update 语句，那么 BeforeCreate 就不会执行，BeforeUpdate 会执行
		require.NoError(t, db.Save(&user).Error)
		require.Len(t, user.logs, 2)
		require.Equal(t, "BeforeSave", user.logs[0])
		require.Equal(t, "BeforeUpdate", user.logs[1])

		user.cleanLogs()
		user.Name = "Charlie"
		require.NoError(t, db.Updates(&user).Error)
		require.Len(t, user.logs, 2)
		require.Equal(t, "BeforeSave", user.logs[0])
		require.Equal(t, "BeforeUpdate", user.logs[1])

		user.cleanLogs()
		// UpdateColumn 不会触发 BeforeUpdate ，并且不会更新 UpdatedAt
		origUpdatedAt := user.UpdatedAt
		require.NoError(t, db.Model(&user).UpdateColumn("name", "David").Error)
		require.Len(t, user.logs, 0)                    // 不会触发 Hooks
		require.Equal(t, origUpdatedAt, user.UpdatedAt) // 不会更新 UpdatedAt
		require.Equal(t, user.Name, "David")            // 会修改字段

		user.cleanLogs()
		require.NoError(t, db.Model(&user).Updates(map[string]any{
			"name": "Eve",
		}).Error)
		require.Len(t, user.logs, 2)                       // 会触发 BeforeSave 和 BeforeUpdate
		require.NotEqual(t, origUpdatedAt, user.UpdatedAt) // 会更新 UpdatedAt
		require.Equal(t, user.Name, "Eve")                 // 会修改字段
		// t.Logf("UpdatedAt: %v", user.UpdatedAt)

		user.cleanLogs()
		origUpdatedAt = user.UpdatedAt
		require.NoError(t, db.Model(&user).Omit("updated_at").Updates(map[string]any{
			"name": "Frank",
		}).Error)
		require.Len(t, user.logs, 2)                    // 会触发 BeforeSave 和 BeforeUpdate
		require.Equal(t, origUpdatedAt, user.UpdatedAt) // 不会更新 UpdatedAt
		require.Equal(t, user.Name, "Frank")            // 会修改字段

		{
			user := &UserWithHook{}
			require.NoError(t, db.Where("name = ?", "Frank").First(user).Error)
			require.Equal(t, "Frank", user.Name)
			require.Equal(t, origUpdatedAt, user.UpdatedAt) // 确实没有更新 UpdatedAt
		}

		{
			require.NoError(t, db.Model(&UserWithHook{}).
				Where("name = ?", "Frank").
				Updates(map[string]any{
					"name": "George",
				}).Error)
			user := &UserWithHook{}
			require.NoError(t, db.Where("name = ?", "George").First(user).Error)
			require.Equal(t, "George", user.Name)
			require.NotEqual(t, origUpdatedAt, user.UpdatedAt) // 即使不以主键更新，也会更新 UpdatedAt
		}

		user.cleanLogs()
		require.NoError(t, db.Delete(&user).Error)
		require.Len(t, user.logs, 1)
		require.Equal(t, "BeforeDelete", user.logs[0])

		require.NoError(t, db.Exec("TRUNCATE TABLE user_with_hooks").Error)
	}

	// 综上所述
	// BeforeSave 在 Save/Create/Updates 时都会执行，BeforeCreate 只会在最终是需要 Create 时执行，BeforeUpdate 只会在最终是需要 Update 时执行
	// BeforeDelete 在 Delete 时才会执行
}
