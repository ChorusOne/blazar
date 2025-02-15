// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.1
// 	protoc        v5.28.2
// source: upgrades_registry.proto

package upgrades_registry

import (
	_ "google.golang.org/genproto/googleapis/api/annotations"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type UpgradeStep int32

const (
	// NONE is the default step of an upgrade. It means that the upgrade is not being executed
	UpgradeStep_NONE UpgradeStep = 0
	// MONITORING means that blazar sees the upcoming upgrade and is monitoring the chain for the upgrade height
	UpgradeStep_MONITORING UpgradeStep = 1
	// DOCKER_COMPOSE_FILE_UPGRADE indicates the blazar is executing the core part of the upgrade vua docker compose
	UpgradeStep_COMPOSE_FILE_UPGRADE UpgradeStep = 2
	// PRE_UPGRADE_CHECK indicates that the blazar is executing the pre-upgrade checks
	UpgradeStep_PRE_UPGRADE_CHECK UpgradeStep = 3
	// POST_UPGRADE_CHECK indicates that the blazar is executing the post-upgrade checks
	UpgradeStep_POST_UPGRADE_CHECK UpgradeStep = 4
)

// Enum value maps for UpgradeStep.
var (
	UpgradeStep_name = map[int32]string{
		0: "NONE",
		1: "MONITORING",
		2: "COMPOSE_FILE_UPGRADE",
		3: "PRE_UPGRADE_CHECK",
		4: "POST_UPGRADE_CHECK",
	}
	UpgradeStep_value = map[string]int32{
		"NONE":                 0,
		"MONITORING":           1,
		"COMPOSE_FILE_UPGRADE": 2,
		"PRE_UPGRADE_CHECK":    3,
		"POST_UPGRADE_CHECK":   4,
	}
)

func (x UpgradeStep) Enum() *UpgradeStep {
	p := new(UpgradeStep)
	*p = x
	return p
}

func (x UpgradeStep) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (UpgradeStep) Descriptor() protoreflect.EnumDescriptor {
	return file_upgrades_registry_proto_enumTypes[0].Descriptor()
}

func (UpgradeStep) Type() protoreflect.EnumType {
	return &file_upgrades_registry_proto_enumTypes[0]
}

func (x UpgradeStep) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use UpgradeStep.Descriptor instead.
func (UpgradeStep) EnumDescriptor() ([]byte, []int) {
	return file_upgrades_registry_proto_rawDescGZIP(), []int{0}
}

type UpgradeStatus int32

const (
	// UNKNOWN is the default status of an upgrade. It means that the status
	UpgradeStatus_UNKNOWN UpgradeStatus = 0
	// SCHEDULED is the initial status of an upgrade. It means that the
	// upgrade is registered with the registry but it's not active yet.
	//
	// An upgrade coming from the chain governance that is still being voted on, is marked as scheduled
	UpgradeStatus_SCHEDULED UpgradeStatus = 1
	// ACTIVE means that the upgrade is acknowledged by network governance or a user and is ready to be executed.
	UpgradeStatus_ACTIVE UpgradeStatus = 2
	// EXECUTING means that the upgrade is currently being executed. The height is reached.
	UpgradeStatus_EXECUTING UpgradeStatus = 3
	// COMPLETED means that the upgrade has been successfully executed.
	UpgradeStatus_COMPLETED UpgradeStatus = 4
	// FAILED means that the upgrade has failed to execute.
	UpgradeStatus_FAILED UpgradeStatus = 5
	// CANCELLED means that the upgrade has been cancelled by a user or the network
	UpgradeStatus_CANCELLED UpgradeStatus = 6
	// EXPIRED means that the upgrade time has passed and blazar did not do anything about it (e.g historical upgrade from the chain governance)
	UpgradeStatus_EXPIRED UpgradeStatus = 7
)

// Enum value maps for UpgradeStatus.
var (
	UpgradeStatus_name = map[int32]string{
		0: "UNKNOWN",
		1: "SCHEDULED",
		2: "ACTIVE",
		3: "EXECUTING",
		4: "COMPLETED",
		5: "FAILED",
		6: "CANCELLED",
		7: "EXPIRED",
	}
	UpgradeStatus_value = map[string]int32{
		"UNKNOWN":   0,
		"SCHEDULED": 1,
		"ACTIVE":    2,
		"EXECUTING": 3,
		"COMPLETED": 4,
		"FAILED":    5,
		"CANCELLED": 6,
		"EXPIRED":   7,
	}
)

func (x UpgradeStatus) Enum() *UpgradeStatus {
	p := new(UpgradeStatus)
	*p = x
	return p
}

func (x UpgradeStatus) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (UpgradeStatus) Descriptor() protoreflect.EnumDescriptor {
	return file_upgrades_registry_proto_enumTypes[1].Descriptor()
}

func (UpgradeStatus) Type() protoreflect.EnumType {
	return &file_upgrades_registry_proto_enumTypes[1]
}

func (x UpgradeStatus) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use UpgradeStatus.Descriptor instead.
func (UpgradeStatus) EnumDescriptor() ([]byte, []int) {
	return file_upgrades_registry_proto_rawDescGZIP(), []int{1}
}

type UpgradeType int32

const (
	// GOVERNANCE is a coordinated upgrade that is initiated by the chain
	// governance. The upgrade is expected to be coordinated across all
	// validators at specific height.
	//
	// Requirements:
	// * there is an onchain governance proposal that has passed
	UpgradeType_GOVERNANCE UpgradeType = 0
	// NON_GOVERNANCE_COORDINATED the upgrade is not coming from the chain,
	// but rather is initiated by the operators.
	//
	// Requirements:
	// * there should be no onchain governance proposal
	// * the upgrade is expected to happen at the same height for all validators (usually it's a state breaking change)
	UpgradeType_NON_GOVERNANCE_COORDINATED UpgradeType = 1
	// NON_GOVERNANCE_UNCOORDINATED the upgrade is not coming from the chain,
	// but rather is initiated by the operators.
	//
	// Requirements:
	// * there should be no onchain governance proposal
	// * the upgrade is not expected to happen at any specific height. Validators are free to upgrade at their own pace. (usually non-state breaking changes)
	UpgradeType_NON_GOVERNANCE_UNCOORDINATED UpgradeType = 2
)

// Enum value maps for UpgradeType.
var (
	UpgradeType_name = map[int32]string{
		0: "GOVERNANCE",
		1: "NON_GOVERNANCE_COORDINATED",
		2: "NON_GOVERNANCE_UNCOORDINATED",
	}
	UpgradeType_value = map[string]int32{
		"GOVERNANCE":                   0,
		"NON_GOVERNANCE_COORDINATED":   1,
		"NON_GOVERNANCE_UNCOORDINATED": 2,
	}
)

func (x UpgradeType) Enum() *UpgradeType {
	p := new(UpgradeType)
	*p = x
	return p
}

func (x UpgradeType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (UpgradeType) Descriptor() protoreflect.EnumDescriptor {
	return file_upgrades_registry_proto_enumTypes[2].Descriptor()
}

func (UpgradeType) Type() protoreflect.EnumType {
	return &file_upgrades_registry_proto_enumTypes[2]
}

func (x UpgradeType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use UpgradeType.Descriptor instead.
func (UpgradeType) EnumDescriptor() ([]byte, []int) {
	return file_upgrades_registry_proto_rawDescGZIP(), []int{2}
}

type ProviderType int32

const (
	// CHAIN means that the upgrade is coming from onchain governance
	ProviderType_CHAIN ProviderType = 0
	// LOCAL means that the upgrade is coming from blazar local storage
	ProviderType_LOCAL ProviderType = 1
	// DATABASE means that the upgrade is coming from the database (e.g PostgreSQL)
	ProviderType_DATABASE ProviderType = 2
)

// Enum value maps for ProviderType.
var (
	ProviderType_name = map[int32]string{
		0: "CHAIN",
		1: "LOCAL",
		2: "DATABASE",
	}
	ProviderType_value = map[string]int32{
		"CHAIN":    0,
		"LOCAL":    1,
		"DATABASE": 2,
	}
)

func (x ProviderType) Enum() *ProviderType {
	p := new(ProviderType)
	*p = x
	return p
}

func (x ProviderType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ProviderType) Descriptor() protoreflect.EnumDescriptor {
	return file_upgrades_registry_proto_enumTypes[3].Descriptor()
}

func (ProviderType) Type() protoreflect.EnumType {
	return &file_upgrades_registry_proto_enumTypes[3]
}

func (x ProviderType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ProviderType.Descriptor instead.
func (ProviderType) EnumDescriptor() ([]byte, []int) {
	return file_upgrades_registry_proto_rawDescGZIP(), []int{3}
}

type Upgrade struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// the height at which the upgrade is expected to happen
	 
	Height int64 `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty" gorm:"primaryKey;not null"`
	// docker image tag
	 
	Tag string `protobuf:"bytes,2,opt,name=tag,proto3" json:"tag,omitempty" gorm:"type:text;not null"`
	// cosmos network name (e.g. cosmoshub) or chain id (e.g. cosmoshub-4)
	 
	Network string `protobuf:"bytes,3,opt,name=network,proto3" json:"network,omitempty" gorm:"primaryKey;type:text;not null"`
	// the short title of the upgrade (e.g. "Coordinated upgrade to v0.42.4 announced on discord channel #announcements")
	 
	Name string `protobuf:"bytes,4,opt,name=name,proto3" json:"name,omitempty" gorm:"type:text;not null"`
	// type of the upgrade (defines what checks and actions should be taken)
	 
	Type UpgradeType `protobuf:"varint,5,opt,name=type,proto3,enum=UpgradeType" json:"type,omitempty" gorm:"not null"`
	// status of the upgrade (DONT set this field manually, it's managed by the registry)
	 
	Status UpgradeStatus `protobuf:"varint,6,opt,name=status,proto3,enum=UpgradeStatus" json:"status,omitempty" gorm:"default:0;not null"`
	// current execution step (DONT set this field manually, it's managed by the registry)
	 
	Step UpgradeStep `protobuf:"varint,7,opt,name=step,proto3,enum=UpgradeStep" json:"step,omitempty" gorm:"default:0;not null"`
	// priority of the upgrade (highest priority wins)
	 
	Priority int32 `protobuf:"varint,8,opt,name=priority,proto3" json:"priority,omitempty" gorm:"primaryKey;not null"`
	// source of the upgrade
	 
	Source ProviderType `protobuf:"varint,9,opt,name=source,proto3,enum=ProviderType" json:"source,omitempty" gorm:"not null"`
	// propoal id associated with the upgrade
	ProposalId *int64 `protobuf:"varint,10,opt,name=proposal_id,json=proposalId,proto3,oneof" json:"proposal_id,omitempty"`
}

func (x *Upgrade) Reset() {
	*x = Upgrade{}
	mi := &file_upgrades_registry_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Upgrade) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Upgrade) ProtoMessage() {}

func (x *Upgrade) ProtoReflect() protoreflect.Message {
	mi := &file_upgrades_registry_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Upgrade.ProtoReflect.Descriptor instead.
func (*Upgrade) Descriptor() ([]byte, []int) {
	return file_upgrades_registry_proto_rawDescGZIP(), []int{0}
}

func (x *Upgrade) GetHeight() int64 {
	if x != nil {
		return x.Height
	}
	return 0
}

func (x *Upgrade) GetTag() string {
	if x != nil {
		return x.Tag
	}
	return ""
}

func (x *Upgrade) GetNetwork() string {
	if x != nil {
		return x.Network
	}
	return ""
}

func (x *Upgrade) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Upgrade) GetType() UpgradeType {
	if x != nil {
		return x.Type
	}
	return UpgradeType_GOVERNANCE
}

func (x *Upgrade) GetStatus() UpgradeStatus {
	if x != nil {
		return x.Status
	}
	return UpgradeStatus_UNKNOWN
}

func (x *Upgrade) GetStep() UpgradeStep {
	if x != nil {
		return x.Step
	}
	return UpgradeStep_NONE
}

func (x *Upgrade) GetPriority() int32 {
	if x != nil {
		return x.Priority
	}
	return 0
}

func (x *Upgrade) GetSource() ProviderType {
	if x != nil {
		return x.Source
	}
	return ProviderType_CHAIN
}

func (x *Upgrade) GetProposalId() int64 {
	if x != nil && x.ProposalId != nil {
		return *x.ProposalId
	}
	return 0
}

// This is the structure of <chain-home>/blazar/upgrades.json
type Upgrades struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Upgrades []*Upgrade `protobuf:"bytes,1,rep,name=upgrades,proto3" json:"upgrades,omitempty"`
}

func (x *Upgrades) Reset() {
	*x = Upgrades{}
	mi := &file_upgrades_registry_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Upgrades) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Upgrades) ProtoMessage() {}

func (x *Upgrades) ProtoReflect() protoreflect.Message {
	mi := &file_upgrades_registry_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Upgrades.ProtoReflect.Descriptor instead.
func (*Upgrades) Descriptor() ([]byte, []int) {
	return file_upgrades_registry_proto_rawDescGZIP(), []int{1}
}

func (x *Upgrades) GetUpgrades() []*Upgrade {
	if x != nil {
		return x.Upgrades
	}
	return nil
}

type AddUpgradeRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The new upgrade to be registered
	Upgrade *Upgrade `protobuf:"bytes,1,opt,name=upgrade,proto3" json:"upgrade,omitempty"`
	// If set to true, the upgrade will be overwritten if it already exists
	Overwrite bool `protobuf:"varint,2,opt,name=overwrite,proto3" json:"overwrite,omitempty"`
}

func (x *AddUpgradeRequest) Reset() {
	*x = AddUpgradeRequest{}
	mi := &file_upgrades_registry_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AddUpgradeRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AddUpgradeRequest) ProtoMessage() {}

func (x *AddUpgradeRequest) ProtoReflect() protoreflect.Message {
	mi := &file_upgrades_registry_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AddUpgradeRequest.ProtoReflect.Descriptor instead.
func (*AddUpgradeRequest) Descriptor() ([]byte, []int) {
	return file_upgrades_registry_proto_rawDescGZIP(), []int{2}
}

func (x *AddUpgradeRequest) GetUpgrade() *Upgrade {
	if x != nil {
		return x.Upgrade
	}
	return nil
}

func (x *AddUpgradeRequest) GetOverwrite() bool {
	if x != nil {
		return x.Overwrite
	}
	return false
}

type AddUpgradeResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *AddUpgradeResponse) Reset() {
	*x = AddUpgradeResponse{}
	mi := &file_upgrades_registry_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AddUpgradeResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AddUpgradeResponse) ProtoMessage() {}

func (x *AddUpgradeResponse) ProtoReflect() protoreflect.Message {
	mi := &file_upgrades_registry_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AddUpgradeResponse.ProtoReflect.Descriptor instead.
func (*AddUpgradeResponse) Descriptor() ([]byte, []int) {
	return file_upgrades_registry_proto_rawDescGZIP(), []int{3}
}

type ListUpgradesRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	DisableCache bool            `protobuf:"varint,1,opt,name=disable_cache,json=disableCache,proto3" json:"disable_cache,omitempty"`
	Height       *int64          `protobuf:"varint,2,opt,name=height,proto3,oneof" json:"height,omitempty"`
	Type         *UpgradeType    `protobuf:"varint,3,opt,name=type,proto3,enum=UpgradeType,oneof" json:"type,omitempty"`
	Source       *ProviderType   `protobuf:"varint,4,opt,name=source,proto3,enum=ProviderType,oneof" json:"source,omitempty"`
	Status       []UpgradeStatus `protobuf:"varint,5,rep,packed,name=status,proto3,enum=UpgradeStatus" json:"status,omitempty"`
	Limit        *int64          `protobuf:"varint,6,opt,name=limit,proto3,oneof" json:"limit,omitempty"`
}

func (x *ListUpgradesRequest) Reset() {
	*x = ListUpgradesRequest{}
	mi := &file_upgrades_registry_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListUpgradesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListUpgradesRequest) ProtoMessage() {}

func (x *ListUpgradesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_upgrades_registry_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListUpgradesRequest.ProtoReflect.Descriptor instead.
func (*ListUpgradesRequest) Descriptor() ([]byte, []int) {
	return file_upgrades_registry_proto_rawDescGZIP(), []int{4}
}

func (x *ListUpgradesRequest) GetDisableCache() bool {
	if x != nil {
		return x.DisableCache
	}
	return false
}

func (x *ListUpgradesRequest) GetHeight() int64 {
	if x != nil && x.Height != nil {
		return *x.Height
	}
	return 0
}

func (x *ListUpgradesRequest) GetType() UpgradeType {
	if x != nil && x.Type != nil {
		return *x.Type
	}
	return UpgradeType_GOVERNANCE
}

func (x *ListUpgradesRequest) GetSource() ProviderType {
	if x != nil && x.Source != nil {
		return *x.Source
	}
	return ProviderType_CHAIN
}

func (x *ListUpgradesRequest) GetStatus() []UpgradeStatus {
	if x != nil {
		return x.Status
	}
	return nil
}

func (x *ListUpgradesRequest) GetLimit() int64 {
	if x != nil && x.Limit != nil {
		return *x.Limit
	}
	return 0
}

type ListUpgradesResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Upgrades []*Upgrade `protobuf:"bytes,1,rep,name=upgrades,proto3" json:"upgrades,omitempty"`
}

func (x *ListUpgradesResponse) Reset() {
	*x = ListUpgradesResponse{}
	mi := &file_upgrades_registry_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListUpgradesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListUpgradesResponse) ProtoMessage() {}

func (x *ListUpgradesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_upgrades_registry_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListUpgradesResponse.ProtoReflect.Descriptor instead.
func (*ListUpgradesResponse) Descriptor() ([]byte, []int) {
	return file_upgrades_registry_proto_rawDescGZIP(), []int{5}
}

func (x *ListUpgradesResponse) GetUpgrades() []*Upgrade {
	if x != nil {
		return x.Upgrades
	}
	return nil
}

type CancelUpgradeRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Height int64        `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`
	Source ProviderType `protobuf:"varint,2,opt,name=source,proto3,enum=ProviderType" json:"source,omitempty"`
	// if set to true, the upgrade is cancelled through the state machine, in this case 'source' is ignored
	Force bool `protobuf:"varint,3,opt,name=force,proto3" json:"force,omitempty"`
}

func (x *CancelUpgradeRequest) Reset() {
	*x = CancelUpgradeRequest{}
	mi := &file_upgrades_registry_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CancelUpgradeRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CancelUpgradeRequest) ProtoMessage() {}

func (x *CancelUpgradeRequest) ProtoReflect() protoreflect.Message {
	mi := &file_upgrades_registry_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CancelUpgradeRequest.ProtoReflect.Descriptor instead.
func (*CancelUpgradeRequest) Descriptor() ([]byte, []int) {
	return file_upgrades_registry_proto_rawDescGZIP(), []int{6}
}

func (x *CancelUpgradeRequest) GetHeight() int64 {
	if x != nil {
		return x.Height
	}
	return 0
}

func (x *CancelUpgradeRequest) GetSource() ProviderType {
	if x != nil {
		return x.Source
	}
	return ProviderType_CHAIN
}

func (x *CancelUpgradeRequest) GetForce() bool {
	if x != nil {
		return x.Force
	}
	return false
}

type CancelUpgradeResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *CancelUpgradeResponse) Reset() {
	*x = CancelUpgradeResponse{}
	mi := &file_upgrades_registry_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CancelUpgradeResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CancelUpgradeResponse) ProtoMessage() {}

func (x *CancelUpgradeResponse) ProtoReflect() protoreflect.Message {
	mi := &file_upgrades_registry_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CancelUpgradeResponse.ProtoReflect.Descriptor instead.
func (*CancelUpgradeResponse) Descriptor() ([]byte, []int) {
	return file_upgrades_registry_proto_rawDescGZIP(), []int{7}
}

// ForceSyncRequest is used to force the registry to sync the upgrades from all registered providers
type ForceSyncRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ForceSyncRequest) Reset() {
	*x = ForceSyncRequest{}
	mi := &file_upgrades_registry_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ForceSyncRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ForceSyncRequest) ProtoMessage() {}

func (x *ForceSyncRequest) ProtoReflect() protoreflect.Message {
	mi := &file_upgrades_registry_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ForceSyncRequest.ProtoReflect.Descriptor instead.
func (*ForceSyncRequest) Descriptor() ([]byte, []int) {
	return file_upgrades_registry_proto_rawDescGZIP(), []int{8}
}

type ForceSyncResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// the height at which the registry is currently synced
	Height int64 `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`
}

func (x *ForceSyncResponse) Reset() {
	*x = ForceSyncResponse{}
	mi := &file_upgrades_registry_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ForceSyncResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ForceSyncResponse) ProtoMessage() {}

func (x *ForceSyncResponse) ProtoReflect() protoreflect.Message {
	mi := &file_upgrades_registry_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ForceSyncResponse.ProtoReflect.Descriptor instead.
func (*ForceSyncResponse) Descriptor() ([]byte, []int) {
	return file_upgrades_registry_proto_rawDescGZIP(), []int{9}
}

func (x *ForceSyncResponse) GetHeight() int64 {
	if x != nil {
		return x.Height
	}
	return 0
}

var File_upgrades_registry_proto protoreflect.FileDescriptor

var file_upgrades_registry_proto_rawDesc = []byte{
	0x0a, 0x17, 0x75, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x73, 0x5f, 0x72, 0x65, 0x67, 0x69, 0x73,
	0x74, 0x72, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xc6, 0x02, 0x0a, 0x07, 0x55, 0x70, 0x67, 0x72,
	0x61, 0x64, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x68, 0x65, 0x69, 0x67, 0x68, 0x74, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x03, 0x52, 0x06, 0x68, 0x65, 0x69, 0x67, 0x68, 0x74, 0x12, 0x10, 0x0a, 0x03, 0x74,
	0x61, 0x67, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x74, 0x61, 0x67, 0x12, 0x18, 0x0a,
	0x07, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07,
	0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x20, 0x0a, 0x04, 0x74,
	0x79, 0x70, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x0c, 0x2e, 0x55, 0x70, 0x67, 0x72,
	0x61, 0x64, 0x65, 0x54, 0x79, 0x70, 0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x26, 0x0a,
	0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x0e, 0x2e,
	0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x06, 0x73,
	0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x20, 0x0a, 0x04, 0x73, 0x74, 0x65, 0x70, 0x18, 0x07, 0x20,
	0x01, 0x28, 0x0e, 0x32, 0x0c, 0x2e, 0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x53, 0x74, 0x65,
	0x70, 0x52, 0x04, 0x73, 0x74, 0x65, 0x70, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x72, 0x69, 0x6f, 0x72,
	0x69, 0x74, 0x79, 0x18, 0x08, 0x20, 0x01, 0x28, 0x05, 0x52, 0x08, 0x70, 0x72, 0x69, 0x6f, 0x72,
	0x69, 0x74, 0x79, 0x12, 0x25, 0x0a, 0x06, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x18, 0x09, 0x20,
	0x01, 0x28, 0x0e, 0x32, 0x0d, 0x2e, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x54, 0x79,
	0x70, 0x65, 0x52, 0x06, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x12, 0x24, 0x0a, 0x0b, 0x70, 0x72,
	0x6f, 0x70, 0x6f, 0x73, 0x61, 0x6c, 0x5f, 0x69, 0x64, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x03, 0x48,
	0x00, 0x52, 0x0a, 0x70, 0x72, 0x6f, 0x70, 0x6f, 0x73, 0x61, 0x6c, 0x49, 0x64, 0x88, 0x01, 0x01,
	0x42, 0x0e, 0x0a, 0x0c, 0x5f, 0x70, 0x72, 0x6f, 0x70, 0x6f, 0x73, 0x61, 0x6c, 0x5f, 0x69, 0x64,
	0x22, 0x30, 0x0a, 0x08, 0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x73, 0x12, 0x24, 0x0a, 0x08,
	0x75, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x08,
	0x2e, 0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x52, 0x08, 0x75, 0x70, 0x67, 0x72, 0x61, 0x64,
	0x65, 0x73, 0x22, 0x55, 0x0a, 0x11, 0x41, 0x64, 0x64, 0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x22, 0x0a, 0x07, 0x75, 0x70, 0x67, 0x72, 0x61,
	0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e, 0x55, 0x70, 0x67, 0x72, 0x61,
	0x64, 0x65, 0x52, 0x07, 0x75, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x6f,
	0x76, 0x65, 0x72, 0x77, 0x72, 0x69, 0x74, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09,
	0x6f, 0x76, 0x65, 0x72, 0x77, 0x72, 0x69, 0x74, 0x65, 0x22, 0x14, 0x0a, 0x12, 0x41, 0x64, 0x64,
	0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x96, 0x02, 0x0a, 0x13, 0x4c, 0x69, 0x73, 0x74, 0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x73,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x23, 0x0a, 0x0d, 0x64, 0x69, 0x73, 0x61, 0x62,
	0x6c, 0x65, 0x5f, 0x63, 0x61, 0x63, 0x68, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0c,
	0x64, 0x69, 0x73, 0x61, 0x62, 0x6c, 0x65, 0x43, 0x61, 0x63, 0x68, 0x65, 0x12, 0x1b, 0x0a, 0x06,
	0x68, 0x65, 0x69, 0x67, 0x68, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x48, 0x00, 0x52, 0x06,
	0x68, 0x65, 0x69, 0x67, 0x68, 0x74, 0x88, 0x01, 0x01, 0x12, 0x25, 0x0a, 0x04, 0x74, 0x79, 0x70,
	0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x0c, 0x2e, 0x55, 0x70, 0x67, 0x72, 0x61, 0x64,
	0x65, 0x54, 0x79, 0x70, 0x65, 0x48, 0x01, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x88, 0x01, 0x01,
	0x12, 0x2a, 0x0a, 0x06, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x0d, 0x2e, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x54, 0x79, 0x70, 0x65, 0x48,
	0x02, 0x52, 0x06, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x88, 0x01, 0x01, 0x12, 0x26, 0x0a, 0x06,
	0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x05, 0x20, 0x03, 0x28, 0x0e, 0x32, 0x0e, 0x2e, 0x55,
	0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x06, 0x73, 0x74,
	0x61, 0x74, 0x75, 0x73, 0x12, 0x19, 0x0a, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x18, 0x06, 0x20,
	0x01, 0x28, 0x03, 0x48, 0x03, 0x52, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x88, 0x01, 0x01, 0x42,
	0x09, 0x0a, 0x07, 0x5f, 0x68, 0x65, 0x69, 0x67, 0x68, 0x74, 0x42, 0x07, 0x0a, 0x05, 0x5f, 0x74,
	0x79, 0x70, 0x65, 0x42, 0x09, 0x0a, 0x07, 0x5f, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x42, 0x08,
	0x0a, 0x06, 0x5f, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x22, 0x3c, 0x0a, 0x14, 0x4c, 0x69, 0x73, 0x74,
	0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x24, 0x0a, 0x08, 0x75, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x08, 0x2e, 0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x52, 0x08, 0x75, 0x70,
	0x67, 0x72, 0x61, 0x64, 0x65, 0x73, 0x22, 0x6b, 0x0a, 0x14, 0x43, 0x61, 0x6e, 0x63, 0x65, 0x6c,
	0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x16,
	0x0a, 0x06, 0x68, 0x65, 0x69, 0x67, 0x68, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x06,
	0x68, 0x65, 0x69, 0x67, 0x68, 0x74, 0x12, 0x25, 0x0a, 0x06, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x0d, 0x2e, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65,
	0x72, 0x54, 0x79, 0x70, 0x65, 0x52, 0x06, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x12, 0x14, 0x0a,
	0x05, 0x66, 0x6f, 0x72, 0x63, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x05, 0x66, 0x6f,
	0x72, 0x63, 0x65, 0x22, 0x17, 0x0a, 0x15, 0x43, 0x61, 0x6e, 0x63, 0x65, 0x6c, 0x55, 0x70, 0x67,
	0x72, 0x61, 0x64, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x12, 0x0a, 0x10,
	0x46, 0x6f, 0x72, 0x63, 0x65, 0x53, 0x79, 0x6e, 0x63, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x22, 0x2b, 0x0a, 0x11, 0x46, 0x6f, 0x72, 0x63, 0x65, 0x53, 0x79, 0x6e, 0x63, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x68, 0x65, 0x69, 0x67, 0x68, 0x74, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x06, 0x68, 0x65, 0x69, 0x67, 0x68, 0x74, 0x2a, 0x70, 0x0a,
	0x0b, 0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x53, 0x74, 0x65, 0x70, 0x12, 0x08, 0x0a, 0x04,
	0x4e, 0x4f, 0x4e, 0x45, 0x10, 0x00, 0x12, 0x0e, 0x0a, 0x0a, 0x4d, 0x4f, 0x4e, 0x49, 0x54, 0x4f,
	0x52, 0x49, 0x4e, 0x47, 0x10, 0x01, 0x12, 0x18, 0x0a, 0x14, 0x43, 0x4f, 0x4d, 0x50, 0x4f, 0x53,
	0x45, 0x5f, 0x46, 0x49, 0x4c, 0x45, 0x5f, 0x55, 0x50, 0x47, 0x52, 0x41, 0x44, 0x45, 0x10, 0x02,
	0x12, 0x15, 0x0a, 0x11, 0x50, 0x52, 0x45, 0x5f, 0x55, 0x50, 0x47, 0x52, 0x41, 0x44, 0x45, 0x5f,
	0x43, 0x48, 0x45, 0x43, 0x4b, 0x10, 0x03, 0x12, 0x16, 0x0a, 0x12, 0x50, 0x4f, 0x53, 0x54, 0x5f,
	0x55, 0x50, 0x47, 0x52, 0x41, 0x44, 0x45, 0x5f, 0x43, 0x48, 0x45, 0x43, 0x4b, 0x10, 0x04, 0x2a,
	0x7d, 0x0a, 0x0d, 0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73,
	0x12, 0x0b, 0x0a, 0x07, 0x55, 0x4e, 0x4b, 0x4e, 0x4f, 0x57, 0x4e, 0x10, 0x00, 0x12, 0x0d, 0x0a,
	0x09, 0x53, 0x43, 0x48, 0x45, 0x44, 0x55, 0x4c, 0x45, 0x44, 0x10, 0x01, 0x12, 0x0a, 0x0a, 0x06,
	0x41, 0x43, 0x54, 0x49, 0x56, 0x45, 0x10, 0x02, 0x12, 0x0d, 0x0a, 0x09, 0x45, 0x58, 0x45, 0x43,
	0x55, 0x54, 0x49, 0x4e, 0x47, 0x10, 0x03, 0x12, 0x0d, 0x0a, 0x09, 0x43, 0x4f, 0x4d, 0x50, 0x4c,
	0x45, 0x54, 0x45, 0x44, 0x10, 0x04, 0x12, 0x0a, 0x0a, 0x06, 0x46, 0x41, 0x49, 0x4c, 0x45, 0x44,
	0x10, 0x05, 0x12, 0x0d, 0x0a, 0x09, 0x43, 0x41, 0x4e, 0x43, 0x45, 0x4c, 0x4c, 0x45, 0x44, 0x10,
	0x06, 0x12, 0x0b, 0x0a, 0x07, 0x45, 0x58, 0x50, 0x49, 0x52, 0x45, 0x44, 0x10, 0x07, 0x2a, 0x5f,
	0x0a, 0x0b, 0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x54, 0x79, 0x70, 0x65, 0x12, 0x0e, 0x0a,
	0x0a, 0x47, 0x4f, 0x56, 0x45, 0x52, 0x4e, 0x41, 0x4e, 0x43, 0x45, 0x10, 0x00, 0x12, 0x1e, 0x0a,
	0x1a, 0x4e, 0x4f, 0x4e, 0x5f, 0x47, 0x4f, 0x56, 0x45, 0x52, 0x4e, 0x41, 0x4e, 0x43, 0x45, 0x5f,
	0x43, 0x4f, 0x4f, 0x52, 0x44, 0x49, 0x4e, 0x41, 0x54, 0x45, 0x44, 0x10, 0x01, 0x12, 0x20, 0x0a,
	0x1c, 0x4e, 0x4f, 0x4e, 0x5f, 0x47, 0x4f, 0x56, 0x45, 0x52, 0x4e, 0x41, 0x4e, 0x43, 0x45, 0x5f,
	0x55, 0x4e, 0x43, 0x4f, 0x4f, 0x52, 0x44, 0x49, 0x4e, 0x41, 0x54, 0x45, 0x44, 0x10, 0x02, 0x2a,
	0x32, 0x0a, 0x0c, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x54, 0x79, 0x70, 0x65, 0x12,
	0x09, 0x0a, 0x05, 0x43, 0x48, 0x41, 0x49, 0x4e, 0x10, 0x00, 0x12, 0x09, 0x0a, 0x05, 0x4c, 0x4f,
	0x43, 0x41, 0x4c, 0x10, 0x01, 0x12, 0x0c, 0x0a, 0x08, 0x44, 0x41, 0x54, 0x41, 0x42, 0x41, 0x53,
	0x45, 0x10, 0x02, 0x32, 0xf5, 0x02, 0x0a, 0x0f, 0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x52,
	0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x12, 0x52, 0x0a, 0x0a, 0x41, 0x64, 0x64, 0x55, 0x70,
	0x67, 0x72, 0x61, 0x64, 0x65, 0x12, 0x12, 0x2e, 0x41, 0x64, 0x64, 0x55, 0x70, 0x67, 0x72, 0x61,
	0x64, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x13, 0x2e, 0x41, 0x64, 0x64, 0x55,
	0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x1b,
	0x82, 0xd3, 0xe4, 0x93, 0x02, 0x15, 0x3a, 0x01, 0x2a, 0x22, 0x10, 0x2f, 0x76, 0x31, 0x2f, 0x75,
	0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x73, 0x2f, 0x61, 0x64, 0x64, 0x12, 0x56, 0x0a, 0x0c, 0x4c,
	0x69, 0x73, 0x74, 0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x73, 0x12, 0x14, 0x2e, 0x4c, 0x69,
	0x73, 0x74, 0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x15, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x73,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x19, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x13,
	0x12, 0x11, 0x2f, 0x76, 0x31, 0x2f, 0x75, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x73, 0x2f, 0x6c,
	0x69, 0x73, 0x74, 0x12, 0x5e, 0x0a, 0x0d, 0x43, 0x61, 0x6e, 0x63, 0x65, 0x6c, 0x55, 0x70, 0x67,
	0x72, 0x61, 0x64, 0x65, 0x12, 0x15, 0x2e, 0x43, 0x61, 0x6e, 0x63, 0x65, 0x6c, 0x55, 0x70, 0x67,
	0x72, 0x61, 0x64, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x43, 0x61,
	0x6e, 0x63, 0x65, 0x6c, 0x55, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x22, 0x1e, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x18, 0x3a, 0x01, 0x2a, 0x22, 0x13,
	0x2f, 0x76, 0x31, 0x2f, 0x75, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x73, 0x2f, 0x63, 0x61, 0x6e,
	0x63, 0x65, 0x6c, 0x12, 0x56, 0x0a, 0x09, 0x46, 0x6f, 0x72, 0x63, 0x65, 0x53, 0x79, 0x6e, 0x63,
	0x12, 0x11, 0x2e, 0x46, 0x6f, 0x72, 0x63, 0x65, 0x53, 0x79, 0x6e, 0x63, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x12, 0x2e, 0x46, 0x6f, 0x72, 0x63, 0x65, 0x53, 0x79, 0x6e, 0x63, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x22, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x1c, 0x3a,
	0x01, 0x2a, 0x22, 0x17, 0x2f, 0x76, 0x31, 0x2f, 0x75, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x73,
	0x2f, 0x66, 0x6f, 0x72, 0x63, 0x65, 0x5f, 0x73, 0x79, 0x6e, 0x63, 0x42, 0x26, 0x5a, 0x24, 0x69,
	0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x2f, 0x75, 0x70, 0x67, 0x72, 0x61, 0x64, 0x65, 0x73, 0x5f, 0x72, 0x65, 0x67, 0x69, 0x73,
	0x74, 0x72, 0x79, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_upgrades_registry_proto_rawDescOnce sync.Once
	file_upgrades_registry_proto_rawDescData = file_upgrades_registry_proto_rawDesc
)

func file_upgrades_registry_proto_rawDescGZIP() []byte {
	file_upgrades_registry_proto_rawDescOnce.Do(func() {
		file_upgrades_registry_proto_rawDescData = protoimpl.X.CompressGZIP(file_upgrades_registry_proto_rawDescData)
	})
	return file_upgrades_registry_proto_rawDescData
}

var file_upgrades_registry_proto_enumTypes = make([]protoimpl.EnumInfo, 4)
var file_upgrades_registry_proto_msgTypes = make([]protoimpl.MessageInfo, 10)
var file_upgrades_registry_proto_goTypes = []any{
	(UpgradeStep)(0),              // 0: UpgradeStep
	(UpgradeStatus)(0),            // 1: UpgradeStatus
	(UpgradeType)(0),              // 2: UpgradeType
	(ProviderType)(0),             // 3: ProviderType
	(*Upgrade)(nil),               // 4: Upgrade
	(*Upgrades)(nil),              // 5: Upgrades
	(*AddUpgradeRequest)(nil),     // 6: AddUpgradeRequest
	(*AddUpgradeResponse)(nil),    // 7: AddUpgradeResponse
	(*ListUpgradesRequest)(nil),   // 8: ListUpgradesRequest
	(*ListUpgradesResponse)(nil),  // 9: ListUpgradesResponse
	(*CancelUpgradeRequest)(nil),  // 10: CancelUpgradeRequest
	(*CancelUpgradeResponse)(nil), // 11: CancelUpgradeResponse
	(*ForceSyncRequest)(nil),      // 12: ForceSyncRequest
	(*ForceSyncResponse)(nil),     // 13: ForceSyncResponse
}
var file_upgrades_registry_proto_depIdxs = []int32{
	2,  // 0: Upgrade.type:type_name -> UpgradeType
	1,  // 1: Upgrade.status:type_name -> UpgradeStatus
	0,  // 2: Upgrade.step:type_name -> UpgradeStep
	3,  // 3: Upgrade.source:type_name -> ProviderType
	4,  // 4: Upgrades.upgrades:type_name -> Upgrade
	4,  // 5: AddUpgradeRequest.upgrade:type_name -> Upgrade
	2,  // 6: ListUpgradesRequest.type:type_name -> UpgradeType
	3,  // 7: ListUpgradesRequest.source:type_name -> ProviderType
	1,  // 8: ListUpgradesRequest.status:type_name -> UpgradeStatus
	4,  // 9: ListUpgradesResponse.upgrades:type_name -> Upgrade
	3,  // 10: CancelUpgradeRequest.source:type_name -> ProviderType
	6,  // 11: UpgradeRegistry.AddUpgrade:input_type -> AddUpgradeRequest
	8,  // 12: UpgradeRegistry.ListUpgrades:input_type -> ListUpgradesRequest
	10, // 13: UpgradeRegistry.CancelUpgrade:input_type -> CancelUpgradeRequest
	12, // 14: UpgradeRegistry.ForceSync:input_type -> ForceSyncRequest
	7,  // 15: UpgradeRegistry.AddUpgrade:output_type -> AddUpgradeResponse
	9,  // 16: UpgradeRegistry.ListUpgrades:output_type -> ListUpgradesResponse
	11, // 17: UpgradeRegistry.CancelUpgrade:output_type -> CancelUpgradeResponse
	13, // 18: UpgradeRegistry.ForceSync:output_type -> ForceSyncResponse
	15, // [15:19] is the sub-list for method output_type
	11, // [11:15] is the sub-list for method input_type
	11, // [11:11] is the sub-list for extension type_name
	11, // [11:11] is the sub-list for extension extendee
	0,  // [0:11] is the sub-list for field type_name
}

func init() { file_upgrades_registry_proto_init() }
func file_upgrades_registry_proto_init() {
	if File_upgrades_registry_proto != nil {
		return
	}
	file_upgrades_registry_proto_msgTypes[0].OneofWrappers = []any{}
	file_upgrades_registry_proto_msgTypes[4].OneofWrappers = []any{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_upgrades_registry_proto_rawDesc,
			NumEnums:      4,
			NumMessages:   10,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_upgrades_registry_proto_goTypes,
		DependencyIndexes: file_upgrades_registry_proto_depIdxs,
		EnumInfos:         file_upgrades_registry_proto_enumTypes,
		MessageInfos:      file_upgrades_registry_proto_msgTypes,
	}.Build()
	File_upgrades_registry_proto = out.File
	file_upgrades_registry_proto_rawDesc = nil
	file_upgrades_registry_proto_goTypes = nil
	file_upgrades_registry_proto_depIdxs = nil
}
