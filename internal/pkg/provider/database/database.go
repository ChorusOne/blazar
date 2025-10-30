package database

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"blazar/internal/pkg/config"
	"blazar/internal/pkg/errors"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
	vrproto "blazar/internal/pkg/proto/version_resolver"
	"blazar/internal/pkg/provider"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type TimestampSerializer struct{}

func (TimestampSerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue any) error {
	if dbValue == nil {
		field.ReflectValueOf(ctx, dst).SetZero()
		return nil
	}

	var t time.Time
	switch v := dbValue.(type) {
	case time.Time:
		t = v
	case string:
		var err error
		t, err = time.Parse("2006-01-02 15:04:05.999999-07:00", v)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported type: %T", dbValue)
	}

	ts := timestamppb.New(t)
	fieldValue := field.ReflectValueOf(ctx, dst)
	fieldValue.Set(reflect.ValueOf(ts))
	return nil
}

func (TimestampSerializer) Value(_ context.Context, _ *schema.Field, dst reflect.Value, fieldValue any) (any, error) {
	ts, ok := fieldValue.(*timestamppb.Timestamp)
	if !ok || ts == nil {
		return nil, nil
	}

	return ts.AsTime(), nil
}

func init() {
	schema.RegisterSerializer("timestamppb", TimestampSerializer{})
}

type Provider struct {
	db       *gorm.DB
	priority int32
	network  string
}

func NewDatabaseProviderWithDB(db *gorm.DB, network string, priority int32) *Provider {
	return &Provider{
		db:       db,
		network:  network,
		priority: priority,
	}
}

func NewDatabaseProvider(cfg *config.DatabaseProvider, network string) (*Provider, error) {
	db, err := InitDB(cfg, &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if cfg.AutoMigrate {
		if err := AutoMigrate(db); err != nil {
			return nil, err
		}
	}

	provider := &Provider{
		db:       db,
		network:  network,
		priority: cfg.DefaultPriority,
	}

	return provider, nil
}

func (dp Provider) GetUpgrades(ctx context.Context) ([]*urproto.Upgrade, error) {
	var dbUpgrades []*urproto.Upgrade
	result := dp.db.WithContext(ctx).Where("network = ?", dp.network).Find(&dbUpgrades)
	if result.Error != nil {
		return []*urproto.Upgrade{}, errors.Wrapf(result.Error, "failed to get upgrades from database")
	}

	return provider.PostProcessUpgrades(dbUpgrades, urproto.ProviderType_DATABASE, dp.priority), nil
}

func (dp Provider) GetUpgradesByType(ctx context.Context, upgradeType urproto.UpgradeType) ([]*urproto.Upgrade, error) {
	var dbUpgrades []*urproto.Upgrade
	result := dp.db.WithContext(ctx).Where("network = ? AND type = ?", dp.network, upgradeType.String()).Find(&dbUpgrades)
	if result.Error != nil {
		return []*urproto.Upgrade{}, errors.Wrapf(result.Error, "failed to get upgrades by type from database")
	}

	return provider.PostProcessUpgrades(dbUpgrades, urproto.ProviderType_DATABASE, dp.priority), nil
}

func (dp Provider) GetUpgradesByHeight(ctx context.Context, height int64) ([]*urproto.Upgrade, error) {
	var upgrades []*urproto.Upgrade
	result := dp.db.WithContext(ctx).Where("height = ? AND network = ?", height, dp.network).Find(&upgrades)
	if result.Error != nil {
		return []*urproto.Upgrade{}, errors.Wrapf(result.Error, "failed to get upgrade by id from database")
	}

	return provider.PostProcessUpgrades(upgrades, urproto.ProviderType_DATABASE, dp.priority), nil
}

func (dp Provider) AddUpgrade(ctx context.Context, upgrade *urproto.Upgrade, overwrite bool) error {
	provider.PostProcessUpgrade(upgrade, urproto.ProviderType_DATABASE, dp.priority)

	// update the entry if exists or create a new one (depends on overwrite flag)
	if overwrite {
		result := dp.db.WithContext(ctx).Clauses(clause.OnConflict{
			// NOTE: this is the compound primary key
			Columns: []clause.Column{{Name: "height"}, {Name: "network"}, {Name: "priority"}},
			// this should include the rest of the columns
			// NOTE: status and step is managed by blazar state machine and should not be updated
			DoUpdates: clause.AssignmentColumns([]string{"tag", "name", "type" /* "status", */ /* step,  */, "source", "proposal_id"}),
		}).Create(upgrade)
		return result.Error
	}

	result := dp.db.Create(upgrade)
	return result.Error
}

func (dp Provider) RegisterVersion(ctx context.Context, version *vrproto.Version, overwrite bool) error {
	provider.PostProcessVersion(version, urproto.ProviderType_DATABASE, dp.priority)

	if overwrite {
		result := dp.db.WithContext(ctx).Clauses(clause.OnConflict{
			// NOTE: this is the compound primary key
			Columns: []clause.Column{{Name: "height"}, {Name: "network"}, {Name: "priority"}},
			// this should include the rest of the columns
			DoUpdates: clause.AssignmentColumns([]string{"tag", "source"}),
		}).Create(version)
		return result.Error
	}

	result := dp.db.WithContext(ctx).Create(version)
	return result.Error
}

func (dp Provider) GetVersions(ctx context.Context) ([]*vrproto.Version, error) {
	var versions []*vrproto.Version

	result := dp.db.WithContext(ctx).Where("network = ?", dp.network).Find(&versions)
	if result.Error != nil {
		return []*vrproto.Version{}, errors.Wrapf(result.Error, "failed to get versions from database")
	}

	return provider.PostProcessVersions(versions, urproto.ProviderType_DATABASE, dp.priority), nil
}

func (dp Provider) GetVersionsByHeight(ctx context.Context, height uint64) ([]*vrproto.Version, error) {
	var versions []*vrproto.Version

	result := dp.db.WithContext(ctx).Where("height = ? AND network = ?", height, dp.network).Find(&versions)
	if result.Error != nil {
		return nil, errors.Wrapf(result.Error, "failed to get version by height from database")
	}

	if versions == nil {
		return nil, fmt.Errorf("version not found for height: %d", height)
	}

	return provider.PostProcessVersions(versions, urproto.ProviderType_DATABASE, dp.priority), nil
}

func (dp Provider) CancelUpgrade(ctx context.Context, height int64, network string) error {
	total := int64(0)
	result := dp.db.WithContext(ctx).Model(&urproto.Upgrade{}).Where("network = ? AND height = ?", dp.network, height).Count(&total)

	if result.Error != nil {
		return errors.Wrapf(result.Error, "failed to count upgrades from database")
	}

	if total == 0 {
		// if there is no upgrades registered (in database provider) blazar will create one with status CANCELLED
		result := dp.db.WithContext(ctx).Model(&urproto.Upgrade{}).Create(&urproto.Upgrade{
			Height:     height,
			Network:    network,
			Priority:   dp.priority,
			Name:       "",
			Type:       urproto.UpgradeType_NON_GOVERNANCE_UNCOORDINATED,
			Status:     urproto.UpgradeStatus_CANCELLED,
			Step:       urproto.UpgradeStep_NONE,
			Source:     urproto.ProviderType_DATABASE,
			ProposalId: nil,
			CreatedAt:  timestamppb.Now(),
		})

		if result.Error != nil {
			return errors.Wrapf(result.Error, "failed to create cancellation upgrade")
		}
	} else {
		// if the upgrade is already registered in the database
		// update the record with highest priority for given height
		//
		// Equivalent SQL query
		// ```
		// UPDATE "upgrades" SET priority = (
		//  SELECT MAX(priority) FROM "upgrades" WHERE height = XXX AND network = 'XXX'
		// ), status=6 WHERE height = XXX AND network = 'XXX'
		// ```
		result := dp.db.WithContext(ctx).Model(&urproto.Upgrade{}).Where(
			"height = ? AND network = ?", height, network,
		).Updates(
			map[string]interface{}{
				"status":   urproto.UpgradeStatus_CANCELLED,
				"priority": dp.db.Model(&urproto.Upgrade{}).Select("MAX(priority)").Where("height = ? AND network = ?", height, network),
			},
		)

		if result.Error != nil {
			return errors.Wrapf(result.Error, "failed to cancel upgrade from database")
		}
	}

	return nil
}

func (dp Provider) Type() urproto.ProviderType {
	return urproto.ProviderType_DATABASE
}

func InitDB(cfg *config.DatabaseProvider, gcfg *gorm.Config) (*gorm.DB, error) {
	mode := string(cfg.SslMode)

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s", cfg.Host, cfg.User, cfg.Password, cfg.DB, cfg.Port, mode)
	db, err := gorm.Open(postgres.Open(dsn), gcfg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect database")
	}

	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&urproto.Upgrade{}); err != nil {
		return errors.Wrapf(err, "database migration failed for upgrades table")
	}

	if err := db.AutoMigrate(&vrproto.Version{}); err != nil {
		return errors.Wrapf(err, "database migration failed for versions table")
	}

	return nil
}
