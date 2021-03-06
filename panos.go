// Package panos interacts with Palo Alto and Panorama devices using the XML API.
package panos

import (
	"encoding/xml"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/scottdware/go-rested"
)

// PaloAlto is a container for our session state.
type PaloAlto struct {
	Host            string
	Key             string
	URI             string
	Platform        string
	Model           string
	Serial          string
	SoftwareVersion string
	DeviceType      string
	Panorama        bool
}

// Devices lists all of the devices in Panorama.
type Devices struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Code    string   `xml:"code,attr"`
	Devices []Serial `xml:"result>devices>entry"`
}

// DeviceGroups lists all of the device-group's in Panorama.
type DeviceGroups struct {
	XMLName xml.Name      `xml:"response"`
	Status  string        `xml:"status,attr"`
	Code    string        `xml:"code,attr"`
	Groups  []DeviceGroup `xml:"result>device-group>entry"`
}

// DeviceGroup contains information about each individual device-group.
type DeviceGroup struct {
	Name    string   `xml:"name,attr"`
	Devices []Serial `xml:"devices>entry"`
}

// Serial contains the serial number of each device in the device-group.
type Serial struct {
	Serial string `xml:"name,attr"`
}

// Tags contains information about all tags on the system.
type Tags struct {
	Tags []Tag
}

// Tag contains information about each individual tag.
type Tag struct {
	Name     string
	Color    string
	Comments string
}

// xmlTags is used for parsing all tags on the system.
type xmlTags struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Code    string   `xml:"code,attr"`
	Tags    []xmlTag `xml:"result>tag>entry"`
}

// xmlTag is used for parsing each individual tag.
type xmlTag struct {
	Name     string `xml:"name,attr"`
	Color    string `xml:"color,omitempty"`
	Comments string `xml:"comments,omitempty"`
}

// authKey holds our API key.
type authKey struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Code    string   `xml:"code,attr"`
	Key     string   `xml:"result>key"`
}

// systemInfo holds basic system information.
type systemInfo struct {
	XMLName         xml.Name `xml:"response"`
	Status          string   `xml:"status,attr"`
	Code            string   `xml:"code,attr"`
	Platform        string   `xml:"result>system>platform-family"`
	Model           string   `xml:"result>system>model"`
	Serial          string   `xml:"result>system>serial"`
	SoftwareVersion string   `xml:"result>system>sw-version"`
}

// panoramaStatus gets the connection status to Panorama.
type panoramaStatus struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Code    string   `xml:"code,attr"`
	Data    string   `xml:"result"`
}

// requestError contains information about any error we get from a request.
type requestError struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Code    string   `xml:"code,attr"`
	Message string   `xml:"result>msg,omitempty"`
}

var (
	headers = map[string]string{
		"Content-Type": "application/xml",
	}

	tagColors = map[string]string{
		"Red":         "color1",
		"Green":       "color2",
		"Blue":        "color3",
		"Yellow":      "color4",
		"Copper":      "color5",
		"Orange":      "color6",
		"Purple":      "color7",
		"Gray":        "color8",
		"Light Green": "color9",
		"Cyan":        "color10",
		"Light Gray":  "color11",
		"Blue Gray":   "color12",
		"Lime":        "color13",
		"Black":       "color14",
		"Gold":        "color15",
		"Brown":       "color16",
	}

	errorCodes = map[string]string{
		"400": "Bad request - Returned when a required parameter is missing, an illegal parameter value is used",
		"403": "Forbidden - Returned for authentication or authorization errors including invalid key, insufficient admin access rights",
		"1":   "Unknown command - The specific config or operational command is not recognized",
		"2":   "Internal error - Check with technical support when seeing these errors",
		"3":   "Internal error - Check with technical support when seeing these errors",
		"4":   "Internal error - Check with technical support when seeing these errors",
		"5":   "Internal error - Check with technical support when seeing these errors",
		"6":   "Bad Xpath - The xpath specified in one or more attributes of the command is invalid. Check the API browser for proper xpath values",
		"7":   "Object not present - Object specified by the xpath is not present. For example, entry[@name=’value’] where no object with name ‘value’ is present",
		"8":   "Object not unique - For commands that operate on a single object, the specified object is not unique",
		"9":   "Internal error - Check with technical support when seeing these errors",
		"10":  "Reference count not zero - Object cannot be deleted as there are other objects that refer to it. For example, address object still in use in policy",
		"11":  "Internal error - Check with technical support when seeing these errors",
		"12":  "Invalid object - Xpath or element values provided are not complete",
		"13":  "Operation failed - A descriptive error message is returned in the response",
		"14":  "Operation not possible - Operation is not possible. For example, moving a rule up one position when it is already at the top",
		"15":  "Operation denied - For example, Admin not allowed to delete own account, Running a command that is not allowed on a passive device",
		"16":  "Unauthorized - The API role does not have access rights to run this query",
		"17":  "Invalid command - Invalid command or parameters",
		"18":  "Malformed command - The XML is malformed",
		"19":  "Success - Command completed successfully",
		"20":  "Success - Command completed successfully",
		"21":  "Internal error - Check with technical support when seeing these errors",
		"22":  "Session timed out - The session for this query timed out",
	}
)

// splitSWVersion
func splitSWVersion(version string) []int {
	re := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
	match := re.FindStringSubmatch(version)
	maj, _ := strconv.Atoi(match[1])
	min, _ := strconv.Atoi(match[2])
	rel, _ := strconv.Atoi(match[3])

	return []int{maj, min, rel}
}

// NewSession sets up our connection to the Palo Alto firewall or Panorama device.
func NewSession(host, user, passwd string) (*PaloAlto, error) {
	var key authKey
	var info systemInfo
	var pan panoramaStatus
	status := false
	deviceType := "panos"
	r := rested.NewRequest()

	resp := r.Send("get", fmt.Sprintf("https://%s/api/?type=keygen&user=%s&password=%s", host, user, passwd), nil, nil, nil)
	if resp.Error != nil {
		return nil, resp.Error
	}

	err := xml.Unmarshal(resp.Body, &key)
	if err != nil {
		return nil, err
	}

	if key.Status != "success" {
		return nil, fmt.Errorf("error code %s: %s (keygen)", key.Code, errorCodes[key.Code])
	}

	uri := fmt.Sprintf("https://%s/api/?", host)
	getInfo := r.Send("get", fmt.Sprintf("%s&key=%s&type=op&cmd=<show><system><info></info></system></show>", uri, key.Key), nil, nil, nil)

	if getInfo.Error != nil {
		return nil, getInfo.Error
	}

	err = xml.Unmarshal(getInfo.Body, &info)
	if err != nil {
		return nil, err
	}

	if info.Status != "success" {
		return nil, fmt.Errorf("error code %s: %s (show system info)", info.Code, errorCodes[info.Code])
	}

	panStatus := r.Send("get", fmt.Sprintf("%s&key=%s&type=op&cmd=<show><panorama-status></panorama-status></show>", uri, key.Key), nil, nil, nil)
	if panStatus.Error != nil {
		return nil, panStatus.Error
	}

	err = xml.Unmarshal(panStatus.Body, &pan)
	if err != nil {
		return nil, err
	}

	if info.Platform == "m" {
		deviceType = "panorama"
	}

	if strings.Contains(pan.Data, ": yes") {
		status = true
	}

	return &PaloAlto{
		Host:            host,
		Key:             key.Key,
		URI:             fmt.Sprintf("https://%s/api/?", host),
		Platform:        info.Platform,
		Model:           info.Model,
		Serial:          info.Serial,
		SoftwareVersion: info.SoftwareVersion,
		DeviceType:      deviceType,
		Panorama:        status,
	}, nil
}

// Devices returns information about all of the devices that are managed by Panorama.
func (p *PaloAlto) Devices() (*Devices, error) {
	var devices Devices
	xpath := "/config/mgt-config/devices"
	// xpath := "/config/devices/entry/vsys/entry/address"
	r := rested.NewRequest()

	if p.DeviceType != "panorama" {
		return nil, errors.New("devices can only be listed from a Panorama device")
	}

	query := map[string]string{
		"type":   "config",
		"action": "get",
		"xpath":  xpath,
		"key":    p.Key,
	}
	devData := r.Send("get", p.URI, nil, headers, query)

	if err := xml.Unmarshal(devData.Body, &devices); err != nil {
		return nil, err
	}

	if devices.Status != "success" {
		return nil, fmt.Errorf("error code %s: %s", devices.Code, errorCodes[devices.Code])
	}

	return &devices, nil
}

// DeviceGroups returns information about all of the device-groups in Panorama, and what devices are
// linked to them.
func (p *PaloAlto) DeviceGroups() (*DeviceGroups, error) {
	var devices DeviceGroups
	xpath := "/config/devices/entry//device-group"
	// xpath := "/config/devices/entry/vsys/entry/address"
	r := rested.NewRequest()

	if p.DeviceType != "panorama" {
		return nil, errors.New("device-groups can only be listed from a Panorama device")
	}

	query := map[string]string{
		"type":   "config",
		"action": "get",
		"xpath":  xpath,
		"key":    p.Key,
	}
	devData := r.Send("get", p.URI, nil, headers, query)

	if err := xml.Unmarshal(devData.Body, &devices); err != nil {
		return nil, err
	}

	if devices.Status != "success" {
		return nil, fmt.Errorf("error code %s: %s", devices.Code, errorCodes[devices.Code])
	}

	return &devices, nil
}

// CreateDeviceGroup will create a new device-group on a Panorama device. You can add devices as well by
// specifying the serial numbers in a string slice ([]string). Use 'nil' if you do not wish to add any.
func (p *PaloAlto) CreateDeviceGroup(name, description string, devices []string) error {
	var xmlBody string
	var xpath string
	var reqError requestError
	r := rested.NewRequest()

	if p.DeviceType == "panos" || p.DeviceType != "panorama" {
		return errors.New("you must be connected to a Panorama device when creating a device-group")
	}

	if p.DeviceType == "panorama" {
		xpath = "/config/devices/entry[@name='localhost.localdomain']/device-group"
		xmlBody = fmt.Sprintf("<entry name=\"%s\">", name)
	}

	if devices != nil {
		xmlBody += "<devices>"
		for _, s := range devices {
			xmlBody += fmt.Sprintf("<entry name=\"%s\"/>", strings.TrimSpace(s))
		}
		xmlBody += "</devices>"
	}

	if description != "" {
		xmlBody += fmt.Sprintf("<description>%s</description>", description)
	}

	xmlBody += "</entry>"

	query := map[string]string{
		"type":    "config",
		"action":  "set",
		"xpath":   xpath,
		"element": xmlBody,
		"key":     p.Key,
	}

	resp := r.Send("post", p.URI, nil, nil, query)
	if resp.Error != nil {
		return resp.Error
	}

	if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
		return err
	}

	if reqError.Status != "success" {
		return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
	}

	return nil
}

// DeleteDeviceGroup will delete the given device-group from Panorama.
func (p *PaloAlto) DeleteDeviceGroup(name string) error {
	var xpath string
	var reqError requestError
	r := rested.NewRequest()

	if p.DeviceType == "panos" || p.DeviceType != "panorama" {
		return errors.New("you must be connected to a Panorama device when deleting a device-group")
	}

	if p.DeviceType == "panorama" {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']", name)
	}

	query := map[string]string{
		"type":   "config",
		"action": "delete",
		"xpath":  xpath,
		"key":    p.Key,
	}

	resp := r.Send("post", p.URI, nil, nil, query)
	if resp.Error != nil {
		return resp.Error
	}

	if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
		return err
	}

	if reqError.Status != "success" {
		return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
	}

	return nil
}

// AddDevice will add a new device to a Panorama. If you specify the optional 'devicegroup' parameter,
// it will also add the device to the given device-group.
func (p *PaloAlto) AddDevice(serial string, devicegroup ...string) error {
	var reqError requestError
	r := rested.NewRequest()

	if p.DeviceType == "panos" || p.DeviceType != "panorama" {
		return errors.New("you must be connected to Panorama when adding devices")
	}

	if p.DeviceType == "panorama" && len(devicegroup) <= 0 {
		xpath := "/config/mgt-config/devices"
		xmlBody := fmt.Sprintf("<entry name=\"%s\"/>", serial)

		query := map[string]string{
			"type":    "config",
			"action":  "set",
			"xpath":   xpath,
			"element": xmlBody,
			"key":     p.Key,
		}

		resp := r.Send("post", p.URI, nil, nil, query)
		if resp.Error != nil {
			return resp.Error
		}

		if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
			return err
		}

		if reqError.Status != "success" {
			return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
		}
	}

	if p.DeviceType == "panorama" && len(devicegroup) > 0 {
		deviceXpath := "/config/mgt-config/devices"
		deviceXMLBody := fmt.Sprintf("<entry name=\"%s\"/>", serial)
		xpath := fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']", devicegroup[0])
		xmlBody := fmt.Sprintf("<devices><entry name=\"%s\"/></devices>", serial)

		deviceQuery := map[string]string{
			"type":    "config",
			"action":  "set",
			"xpath":   deviceXpath,
			"element": deviceXMLBody,
			"key":     p.Key,
		}

		addResp := r.Send("post", p.URI, nil, nil, deviceQuery)
		if addResp.Error != nil {
			return addResp.Error
		}

		if err := xml.Unmarshal(addResp.Body, &reqError); err != nil {
			return err
		}

		if reqError.Status != "success" {
			return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
		}

		time.Sleep(200 * time.Millisecond)

		query := map[string]string{
			"type":    "config",
			"action":  "set",
			"xpath":   xpath,
			"element": xmlBody,
			"key":     p.Key,
		}

		resp := r.Send("post", p.URI, nil, nil, query)
		if resp.Error != nil {
			return resp.Error
		}

		if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
			return err
		}

		if reqError.Status != "success" {
			return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
		}
	}

	return nil
}

// SetPanoramaServer will configure a device to be managed by the given Panorama server's IP address.
func (p *PaloAlto) SetPanoramaServer(ip string) error {
	var reqError requestError
	r := rested.NewRequest()
	xpath := "/config/devices/entry[@name='localhost.localdomain']/deviceconfig/system"
	xmlBody := fmt.Sprintf("<panorama-server>%s</panorama-server>", ip)

	if p.DeviceType == "panorama" && p.Panorama == true {
		return errors.New("you must be connected to a non-Panorama device in order to configure a Panorama server")
	}

	query := map[string]string{
		"type":    "config",
		"action":  "set",
		"xpath":   xpath,
		"key":     p.Key,
		"element": xmlBody,
	}

	resp := r.Send("post", p.URI, nil, nil, query)
	if resp.Error != nil {
		return resp.Error
	}

	if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
		return err
	}

	if reqError.Status != "success" {
		return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
	}

	return nil
}

// RemoveDevice will remove a device from Panorama. If you specify the optional 'devicegroup' parameter,
// it will only remove the device from the given device-group.
func (p *PaloAlto) RemoveDevice(serial string, devicegroup ...string) error {
	var xpath string
	var reqError requestError
	r := rested.NewRequest()

	if p.DeviceType == "panos" || p.DeviceType != "panorama" {
		return errors.New("you must be connected to Panorama when removing devices")
	}

	if p.DeviceType == "panorama" && len(devicegroup) <= 0 {
		xpath = fmt.Sprintf("/config/mgt-config/devices/entry[@name='%s']", serial)
	}

	if p.DeviceType == "panorama" && len(devicegroup) > 0 {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/devices/entry[@name='%s']", devicegroup[0], serial)
	}

	query := map[string]string{
		"type":   "config",
		"action": "delete",
		"xpath":  xpath,
		"key":    p.Key,
	}

	resp := r.Send("post", p.URI, nil, nil, query)
	if resp.Error != nil {
		return resp.Error
	}

	if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
		return err
	}

	if reqError.Status != "success" {
		return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
	}

	return nil
}

// Tags returns information about all tags on the system.
func (p *PaloAlto) Tags() (*Tags, error) {
	var parsedTags xmlTags
	var tags Tags
	var tcolor string
	xpath := "/config/devices/entry//tag"
	// xpath := "/config/devices/entry/vsys/entry/tag"
	r := rested.NewRequest()

	if p.DeviceType == "panos" && p.Panorama == true {
		xpath = "/config/panorama//tag"
	}

	if p.DeviceType == "panorama" {
		// xpath = "/config/devices/entry/device-group/entry/tag"
		xpath = "/config/devices/entry//tag"
	}

	query := map[string]string{
		"type":   "config",
		"action": "get",
		"xpath":  xpath,
		"key":    p.Key,
	}
	tData := r.Send("get", p.URI, nil, headers, query)

	if err := xml.Unmarshal(tData.Body, &parsedTags); err != nil {
		return nil, err
	}

	if parsedTags.Status != "success" {
		return nil, fmt.Errorf("error code %s: %s", parsedTags.Code, errorCodes[parsedTags.Code])
	}

	for _, t := range parsedTags.Tags {
		tname := t.Name
		for k, v := range tagColors {
			if t.Color == v {
				tcolor = k
			}
		}
		// tcolor := tagColors[t.Color]
		tcomments := t.Comments

		tags.Tags = append(tags.Tags, Tag{Name: tname, Color: tcolor, Comments: tcomments})
	}

	return &tags, nil

}

// CreateTag will add a new tag to the device. You can use the following colors: Red, Green, Blue, Yellow, Copper,
// Orange, Purple, Gray, Light Green, Cyan, Light Gray, Blue Gray, Lime, Black, Gold, Brown. If creating a tag on a
// Panorama device, then specify the given device-group name as the last parameter.
func (p *PaloAlto) CreateTag(name, color, comments string, devicegroup ...string) error {
	var xmlBody string
	var xpath string
	var reqError requestError
	r := rested.NewRequest()

	xmlBody = fmt.Sprintf("<color>%s</color>", tagColors[color])

	if comments != "" {
		xmlBody += fmt.Sprintf("<comments>%s</comments>", comments)
	}

	if p.DeviceType == "panos" {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/tag/entry[@name='%s']", name)
	}

	if p.DeviceType == "panorama" && len(devicegroup) > 0 {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/tag/entry[@name='%s']", devicegroup[0], name)
	}

	if p.DeviceType == "panorama" && len(devicegroup) <= 0 {
		return errors.New("you must specify a device-group when connected to a Panorama device")
	}

	query := map[string]string{
		"type":    "config",
		"action":  "set",
		"xpath":   xpath,
		"element": xmlBody,
		"key":     p.Key,
	}

	resp := r.Send("post", p.URI, nil, nil, query)
	if resp.Error != nil {
		return resp.Error
	}

	if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
		return err
	}

	if reqError.Status != "success" {
		return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
	}

	return nil
}

// DeleteTag will remove a tag from the device. If deleting a tag on a
// Panorama device, then specify the given device-group name as the last parameter.
func (p *PaloAlto) DeleteTag(name string, devicegroup ...string) error {
	var xpath string
	var reqError requestError
	r := rested.NewRequest()

	if p.DeviceType == "panos" {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/tag/entry[@name='%s']", name)
	}

	if p.DeviceType == "panorama" && len(devicegroup) > 0 {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/tag/entry[@name='%s']", devicegroup[0], name)
	}

	if p.DeviceType == "panorama" && len(devicegroup) <= 0 {
		return errors.New("you must specify a device-group when connected to a Panorama device")
	}

	query := map[string]string{
		"type":   "config",
		"action": "delete",
		"xpath":  xpath,
		"key":    p.Key,
	}

	resp := r.Send("get", p.URI, nil, nil, query)
	if resp.Error != nil {
		return resp.Error
	}

	if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
		return err
	}

	if reqError.Status != "success" {
		return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
	}

	return nil
}

// ApplyTag will apply the given tag to the specified address or service object(s). You can specify multiple tags
// by separating them with a comma, i.e. "servers, vm". If you have address/service objects with the same
// name, then the tag(s) will be applied to all that match. When tagging an object on a Panorama device, specify
// the given device-group name as the last parameter.
func (p *PaloAlto) ApplyTag(tag, object string, devicegroup ...string) error {
	var xpath string
	var reqError requestError
	r := rested.NewRequest()
	tags := strings.Split(tag, ",")
	adObj, _ := p.Addresses()
	agObj, _ := p.AddressGroups()
	sObj, _ := p.Services()
	sgObj, _ := p.ServiceGroups()

	xmlBody := "<tag>"
	for _, t := range tags {
		xmlBody += fmt.Sprintf("<member>%s</member>", strings.TrimSpace(t))
	}
	xmlBody += "</tag>"

	query := map[string]string{
		"type":    "config",
		"action":  "edit",
		"element": xmlBody,
		"key":     p.Key,
	}

	for _, a := range adObj.Addresses {
		if object == a.Name {
			if p.DeviceType == "panos" {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/address/entry[@name='%s']/tag", object)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) > 0 {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/address/entry[@name='%s']/tag", devicegroup[0], object)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) <= 0 {
				return errors.New("you must specify a device-group when connected to a Panorama device")
			}
		}
	}

	for _, ag := range agObj.Groups {
		if object == ag.Name {
			if p.DeviceType == "panos" {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/address-group/entry[@name='%s']/tag", object)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) > 0 {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/address-group/entry[@name='%s']/tag", devicegroup[0], object)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) <= 0 {
				return errors.New("you must specify a device-group when connected to a Panorama device")
			}
		}
	}

	for _, s := range sObj.Services {
		if object == s.Name {
			if p.DeviceType == "panos" {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/service/entry[@name='%s']/tag", object)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) > 0 {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/service/entry[@name='%s']/tag", devicegroup[0], object)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) <= 0 {
				return errors.New("you must specify a device-group when connected to a Panorama device")
			}
		}
	}

	for _, sg := range sgObj.Groups {
		if object == sg.Name {
			if p.DeviceType == "panos" {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/service-group/entry[@name='%s']/tag", object)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) > 0 {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/service-group/entry[@name='%s']/tag", devicegroup[0], object)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) <= 0 {
				return errors.New("you must specify a device-group when connected to a Panorama device")
			}
		}
	}

	return nil
}

// RemoveTag will remove a single tag from an address/service object. If deleting a tag from an object on a
// Panorama device, then specify the given device-group name as the last parameter.
func (p *PaloAlto) RemoveTag(tag, object string, devicegroup ...string) error {
	var xpath string
	var reqError requestError
	r := rested.NewRequest()
	adObj, _ := p.Addresses()
	agObj, _ := p.AddressGroups()
	sObj, _ := p.Services()
	sgObj, _ := p.ServiceGroups()

	query := map[string]string{
		"type":   "config",
		"action": "delete",
		"key":    p.Key,
	}

	for _, a := range adObj.Addresses {
		if object == a.Name {
			if p.DeviceType == "panos" {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/address/entry[@name='%s']/tag/member[text()='%s']", object, tag)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) > 0 {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/address/entry[@name='%s']/tag/member[text()='%s']", devicegroup[0], object, tag)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) <= 0 {
				return errors.New("you must specify a device-group when connected to a Panorama device")
			}
		}
	}

	for _, ag := range agObj.Groups {
		if object == ag.Name {
			if p.DeviceType == "panos" {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/address-group/entry[@name='%s']/tag/member[text()='%s']", object, tag)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) > 0 {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/address-group/entry[@name='%s']/tag/member[text()='%s']", devicegroup[0], object, tag)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) <= 0 {
				return errors.New("you must specify a device-group when connected to a Panorama device")
			}
		}
	}

	for _, s := range sObj.Services {
		if object == s.Name {
			if p.DeviceType == "panos" {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/service/entry[@name='%s']/tag/member[text()='%s']", object, tag)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) > 0 {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/service/entry[@name='%s']/tag/member[text()='%s']", devicegroup[0], object, tag)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) <= 0 {
				return errors.New("you must specify a device-group when connected to a Panorama device")
			}
		}
	}

	for _, sg := range sgObj.Groups {
		if object == sg.Name {
			if p.DeviceType == "panos" {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/service-group/entry[@name='%s']/tag/member[text()='%s']", object, tag)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) > 0 {
				xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/service-group/entry[@name='%s']/tag/member[text()='%s']", devicegroup[0], object, tag)

				query["xpath"] = xpath

				resp := r.Send("post", p.URI, nil, nil, query)
				if resp.Error != nil {
					return resp.Error
				}

				if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
					return err
				}

				if reqError.Status != "success" {
					return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
				}

				return nil
			}

			if p.DeviceType == "panorama" && len(devicegroup) <= 0 {
				return errors.New("you must specify a device-group when connected to a Panorama device")
			}
		}
	}

	return nil
}

// Commit issues a commit on the device. When issuing a commit against a Panorama device,
// the configuration will only be committed to Panorama, and not an individual device-group.
func (p *PaloAlto) Commit() error {
	var reqError requestError
	r := rested.NewRequest()

	query := map[string]string{
		"type": "commit",
		"cmd":  "<commit></commit>",
		"key":  p.Key,
	}

	resp := r.Send("get", p.URI, nil, nil, query)
	if resp.Error != nil {
		return resp.Error
	}

	if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
		return err
	}

	if reqError.Status != "success" {
		return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
	}

	return nil
}

// CommitAll issues a commit to a Panorama device, with the given 'devicegroup.' You can (optionally) specify
// individual devices within that device group by adding each serial number as an additional parameter.
func (p *PaloAlto) CommitAll(devicegroup string, devices ...string) error {
	var reqError requestError
	var cmd string

	r := rested.NewRequest()

	if p.DeviceType == "panorama" && len(devices) <= 0 {
		cmd = fmt.Sprintf("<commit-all><shared-policy><device-group><entry name=\"%s\"/></device-group></shared-policy></commit-all>", devicegroup)
	}

	if p.DeviceType == "panorama" && len(devices) > 0 {
		cmd = fmt.Sprintf("<commit-all><shared-policy><device-group><name>%s</name><devices>", devicegroup)

		for _, d := range devices {
			cmd += fmt.Sprintf("<entry name=\"%s\"/>", d)
		}

		cmd += "</devices></device-group></shared-policy></commit-all>"
	}

	query := map[string]string{
		"type":   "commit",
		"action": "all",
		"cmd":    cmd,
		"key":    p.Key,
	}

	resp := r.Send("get", p.URI, nil, nil, query)
	if resp.Error != nil {
		return resp.Error
	}

	if err := xml.Unmarshal(resp.Body, &reqError); err != nil {
		return err
	}

	if reqError.Status != "success" {
		return fmt.Errorf("error code %s: %s", reqError.Code, errorCodes[reqError.Code])
	}

	return nil
}
