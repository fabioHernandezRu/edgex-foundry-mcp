package edgex

import "encoding/json"

// DTOs mirror the subset of go-mod-core-contracts/v4 (v4.0.3) fields this
// project uses. JSON tags match the upstream DTOs exactly.

// BaseResponse is the common EdgeX response envelope, also used for errors.
type BaseResponse struct {
	APIVersion string `json:"apiVersion"`
	RequestID  string `json:"requestId,omitempty"`
	Message    string `json:"message,omitempty"`
	StatusCode int    `json:"statusCode"`
}

// PingResponse is returned by GET /api/v3/ping.
type PingResponse struct {
	APIVersion  string `json:"apiVersion"`
	Timestamp   string `json:"timestamp"`
	ServiceName string `json:"serviceName"`
}

// VersionResponse is returned by GET /api/v3/version.
type VersionResponse struct {
	APIVersion  string `json:"apiVersion"`
	Version     string `json:"version"`
	ServiceName string `json:"serviceName"`
	SDKVersion  string `json:"sdk_version,omitempty"`
}

// DeviceService is a registered device service.
type DeviceService struct {
	Created     int64          `json:"created,omitempty"`
	Modified    int64          `json:"modified,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Labels      []string       `json:"labels,omitempty"`
	BaseAddress string         `json:"baseAddress"`
	AdminState  string         `json:"adminState"`
	Properties  map[string]any `json:"properties,omitempty"`
}

// AutoEvent schedules readings for a device resource or command.
type AutoEvent struct {
	Interval          string  `json:"interval"`
	OnChange          bool    `json:"onChange"`
	OnChangeThreshold float64 `json:"onChangeThreshold,omitempty"`
	SourceName        string  `json:"sourceName"`
}

// Device is a device registered in core-metadata. Protocols, Properties and
// Tags are free-form and may carry credentials.
type Device struct {
	Created        int64                     `json:"created,omitempty"`
	Modified       int64                     `json:"modified,omitempty"`
	Name           string                    `json:"name"`
	Parent         string                    `json:"parent,omitempty"`
	Description    string                    `json:"description,omitempty"`
	AdminState     string                    `json:"adminState"`
	OperatingState string                    `json:"operatingState"`
	Labels         []string                  `json:"labels,omitempty"`
	Location       any                       `json:"location,omitempty"`
	ServiceName    string                    `json:"serviceName"`
	ProfileName    string                    `json:"profileName,omitempty"`
	AutoEvents     []AutoEvent               `json:"autoEvents,omitempty"`
	Protocols      map[string]map[string]any `json:"protocols"`
	Tags           map[string]any            `json:"tags,omitempty"`
	Properties     map[string]any            `json:"properties"`
}

// ProfileBasicInfo is the summary part of a device profile.
type ProfileBasicInfo struct {
	Created           int64    `json:"created,omitempty"`
	Modified          int64    `json:"modified,omitempty"`
	Name              string   `json:"name"`
	Manufacturer      string   `json:"manufacturer,omitempty"`
	Description       string   `json:"description,omitempty"`
	Model             string   `json:"model,omitempty"`
	Labels            []string `json:"labels,omitempty"`
	LinkedDeviceCount int64    `json:"linkedDeviceCount"`
}

// ResourceProperties describes the value of a device resource.
type ResourceProperties struct {
	ValueType    string   `json:"valueType"`
	ReadWrite    string   `json:"readWrite"`
	Units        string   `json:"units,omitempty"`
	Minimum      *float64 `json:"minimum,omitempty"`
	Maximum      *float64 `json:"maximum,omitempty"`
	DefaultValue string   `json:"defaultValue,omitempty"`
	MediaType    string   `json:"mediaType,omitempty"`
}

// DeviceResource is one resource of a device profile.
type DeviceResource struct {
	Description string             `json:"description,omitempty"`
	Name        string             `json:"name"`
	IsHidden    bool               `json:"isHidden"`
	Attributes  map[string]any     `json:"attributes,omitempty"`
	Properties  ResourceProperties `json:"properties"`
}

// ResourceOperation references a resource from a device command.
type ResourceOperation struct {
	DeviceResource string `json:"deviceResource"`
	DefaultValue   string `json:"defaultValue,omitempty"`
}

// DeviceCommand groups resource operations under one command name.
type DeviceCommand struct {
	Name               string              `json:"name"`
	IsHidden           bool                `json:"isHidden"`
	ReadWrite          string              `json:"readWrite"`
	ResourceOperations []ResourceOperation `json:"resourceOperations"`
}

// DeviceProfile is a full device profile.
type DeviceProfile struct {
	ProfileBasicInfo
	DeviceResources []DeviceResource `json:"deviceResources"`
	DeviceCommands  []DeviceCommand  `json:"deviceCommands,omitempty"`
}

// Reading is one reading stored by core-data. Exactly one of Value,
// BinaryValue or ObjectValue is meaningful, depending on ValueType.
type Reading struct {
	ID           string          `json:"id,omitempty"`
	Origin       int64           `json:"origin"`
	DeviceName   string          `json:"deviceName"`
	ResourceName string          `json:"resourceName"`
	ProfileName  string          `json:"profileName"`
	ValueType    string          `json:"valueType"`
	Units        string          `json:"units,omitempty"`
	Value        *string         `json:"value,omitempty"`
	BinaryValue  []byte          `json:"binaryValue,omitempty"`
	MediaType    string          `json:"mediaType,omitempty"`
	ObjectValue  json.RawMessage `json:"objectValue,omitempty"`
}

// Event groups readings produced together.
type Event struct {
	ID          string    `json:"id"`
	DeviceName  string    `json:"deviceName"`
	ProfileName string    `json:"profileName"`
	SourceName  string    `json:"sourceName"`
	Origin      int64     `json:"origin"`
	Readings    []Reading `json:"readings"`
}

// CoreCommandParameter is one parameter of a core command.
type CoreCommandParameter struct {
	ResourceName string `json:"resourceName"`
	ValueType    string `json:"valueType"`
}

// CoreCommand is a command exposed by core-command.
type CoreCommand struct {
	Name       string                 `json:"name"`
	Get        bool                   `json:"get,omitempty"`
	Set        bool                   `json:"set,omitempty"`
	Path       string                 `json:"path,omitempty"`
	URL        string                 `json:"url,omitempty"`
	Parameters []CoreCommandParameter `json:"parameters,omitempty"`
}

// DeviceCoreCommand lists the core commands of one device.
type DeviceCoreCommand struct {
	DeviceName   string        `json:"deviceName"`
	ProfileName  string        `json:"profileName"`
	CoreCommands []CoreCommand `json:"coreCommands,omitempty"`
}

type multiDeviceServicesResponse struct {
	TotalCount int             `json:"totalCount"`
	Services   []DeviceService `json:"services"`
}

type multiDevicesResponse struct {
	TotalCount int      `json:"totalCount"`
	Devices    []Device `json:"devices"`
}

type deviceResponse struct {
	Device Device `json:"device"`
}

type multiProfilesBasicResponse struct {
	TotalCount int                `json:"totalCount"`
	Profiles   []ProfileBasicInfo `json:"profiles"`
}

type deviceProfileResponse struct {
	Profile DeviceProfile `json:"profile"`
}

type multiReadingsResponse struct {
	TotalCount int       `json:"totalCount"`
	Readings   []Reading `json:"readings"`
}

type countResponse struct {
	Count int64 `json:"count"`
}

type multiDeviceCoreCommandsResponse struct {
	TotalCount         int                 `json:"totalCount"`
	DeviceCoreCommands []DeviceCoreCommand `json:"deviceCoreCommands"`
}

type deviceCoreCommandResponse struct {
	DeviceCoreCommand DeviceCoreCommand `json:"deviceCoreCommand"`
}

type eventResponse struct {
	Event *Event `json:"event"`
}
