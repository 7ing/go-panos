package panos

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"

	"github.com/scottdware/go-rested"
)

// AddressObjects contains a slice of all address objects.
type AddressObjects struct {
	XMLName   xml.Name  `xml:"response"`
	Status    string    `xml:"status,attr"`
	Code      string    `xml:"code,attr"`
	Addresses []Address `xml:"result>address>entry"`
}

// Address contains information about each individual address object.
type Address struct {
	Name        string `xml:"name,attr"`
	IPAddress   string `xml:"ip-netmask,omitempty"`
	IPRange     string `xml:"ip-range,omitempty"`
	FQDN        string `xml:"fqdn,omitempty"`
	Description string `xml:"description,omitempty"`
}

// AddressGroups contains a slice of all address groups.
type AddressGroups struct {
	Groups []AddressGroup
}

// AddressGroup contains information about each individual address group.
type AddressGroup struct {
	Name          string
	Type          string
	Members       []string
	DynamicFilter string
	Description   string
}

// xmlAddressGroups is used for parsing of all address groups.
type xmlAddressGroups struct {
	XMLName xml.Name          `xml:"response"`
	Status  string            `xml:"status,attr"`
	Code    string            `xml:"code,attr"`
	Groups  []xmlAddressGroup `xml:"result>address-group>entry"`
}

// xmlAddressGroup is used for parsing each individual address group.
type xmlAddressGroup struct {
	Name          string   `xml:"name,attr"`
	Members       []string `xml:"static>member,omitempty"`
	DynamicFilter string   `xml:"dynamic>filter,omitempty"`
	Description   string   `xml:"description,omitempty"`
}

// Addresses returns information about all of the address objects. You can (optionally) specify a device-group
// when ran against a Panorama device. If no device-group is specified, then all objects are returned.
func (p *PaloAlto) Addresses(devicegroup ...string) (*AddressObjects, error) {
	var addrs AddressObjects
	xpath := "/config/devices/entry//address"
	r := rested.NewRequest()

	if p.DeviceType != "panorama" && len(devicegroup) > 0 {
		return nil, errors.New("you must be connected to a Panorama device when specifying a device-group")
	}

	if p.DeviceType == "panos" && p.Panorama == true {
		xpath = "/config/panorama//address"
	}

	if p.DeviceType == "panos" && p.Panorama == false {
		xpath = "/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/address"
	}

	if p.DeviceType == "panorama" && len(devicegroup) > 0 {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/address", devicegroup[0])
	}

	query := map[string]string{
		"type":   "config",
		"action": "get",
		"xpath":  xpath,
		"key":    p.Key,
	}
	addrData := r.Send("get", p.URI, nil, headers, query)

	if err := xml.Unmarshal(addrData.Body, &addrs); err != nil {
		return nil, err
	}

	if addrs.Status != "success" {
		return nil, fmt.Errorf("error code %s: %s", addrs.Code, errorCodes[addrs.Code])
	}

	return &addrs, nil
}

// AddressGroups returns information about all of the address groups. You can (optionally) specify a device-group
// when ran against a Panorama device. If no device-group is specified, then all address groups are returned.
func (p *PaloAlto) AddressGroups(devicegroup ...string) (*AddressGroups, error) {
	var parsedGroups xmlAddressGroups
	var groups AddressGroups
	xpath := "/config/devices/entry//address-group"
	r := rested.NewRequest()

	if p.DeviceType != "panorama" && len(devicegroup) > 0 {
		return nil, errors.New("you must be connected to a Panorama device when specifying a device-group")
	}

	if p.DeviceType == "panos" && p.Panorama == true {
		xpath = "/config/panorama//address-group"
	}

	if p.DeviceType == "panos" && p.Panorama == false {
		xpath = "/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/address-group"
	}

	if p.DeviceType == "panorama" && len(devicegroup) > 0 {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/address-group", devicegroup[0])
	}

	query := map[string]string{
		"type":   "config",
		"action": "get",
		"xpath":  xpath,
		"key":    p.Key,
	}
	groupData := r.Send("get", p.URI, nil, headers, query)

	if err := xml.Unmarshal(groupData.Body, &parsedGroups); err != nil {
		return nil, err
	}

	if parsedGroups.Status != "success" {
		return nil, fmt.Errorf("error code %s: %s", parsedGroups.Code, errorCodes[parsedGroups.Code])
	}

	for _, g := range parsedGroups.Groups {
		gname := g.Name
		gtype := "Static"
		gmembers := g.Members
		gfilter := strings.TrimSpace(g.DynamicFilter)
		gdesc := g.Description

		if g.DynamicFilter != "" {
			gtype = "Dynamic"
		}

		groups.Groups = append(groups.Groups, AddressGroup{Name: gname, Type: gtype, Members: gmembers, DynamicFilter: gfilter, Description: gdesc})
	}

	return &groups, nil
}

// CreateAddress will add a new address object to the device. addrtype should be one of: ip, range, or fqdn. If creating
// an address object on a Panorama device, then specify the given device-group name as the last parameter.
func (p *PaloAlto) CreateAddress(name, addrtype, address, description string, devicegroup ...string) error {
	var xmlBody string
	var xpath string
	var reqError requestError
	r := rested.NewRequest()

	switch addrtype {
	case "ip":
		xmlBody = fmt.Sprintf("<ip-netmask>%s</ip-netmask>", address)
	case "range":
		xmlBody = fmt.Sprintf("<ip-range>%s</ip-range>", address)
	case "fqdn":
		xmlBody = fmt.Sprintf("<fqdn>%s</fqdn>", address)
	}

	if description != "" {
		xmlBody += fmt.Sprintf("<description>%s</description>", description)
	}

	if p.DeviceType == "panos" {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/address/entry[@name='%s']", name)
	}

	if p.DeviceType == "panorama" && len(devicegroup) > 0 {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/address/entry[@name='%s']", devicegroup[0], name)
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

// CreateSharedAddress will add a new shared address object to Panorama. addrtype should be one of: ip, range, or fqdn.
func (p *PaloAlto) CreateSharedAddress(name, addrtype, address, description string) error {
	var xmlBody string
	var xpath string
	var reqError requestError
	r := rested.NewRequest()

	switch addrtype {
	case "ip":
		xmlBody = fmt.Sprintf("<ip-netmask>%s</ip-netmask>", address)
	case "range":
		xmlBody = fmt.Sprintf("<ip-range>%s</ip-range>", address)
	case "fqdn":
		xmlBody = fmt.Sprintf("<fqdn>%s</fqdn>", address)
	}

	if description != "" {
		xmlBody += fmt.Sprintf("<description>%s</description>", description)
	}

	if p.DeviceType == "panos" {
		return errors.New("you can only create shared objects when connected to a Panorama device")
	}

	if p.DeviceType == "panorama" {
		xpath = fmt.Sprintf("/config/shared/address/entry[@name='%s']", name)
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

// CreateStaticGroup will create a new static address group on the device. You can specify multiple members
// by separating them with a comma, i.e. "web-server1, web-server2". If creating an address group on
// a Panorama device, then specify the given device-group name as the last parameter.
func (p *PaloAlto) CreateStaticGroup(name, members, description string, devicegroup ...string) error {
	var xmlBody string
	var xpath string
	var reqError requestError
	r := rested.NewRequest()
	m := strings.Split(members, ",")

	if members == "" {
		return errors.New("you cannot create a static address group without any members")
	}

	xmlBody = "<static>"
	for _, member := range m {
		xmlBody += fmt.Sprintf("<member>%s</member>", strings.TrimSpace(member))
	}
	xmlBody += "</static>"

	if description != "" {
		xmlBody += fmt.Sprintf("<description>%s</description>", description)
	}

	if p.DeviceType == "panos" {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/address-group/entry[@name='%s']", name)
	}

	if p.DeviceType == "panorama" && len(devicegroup) > 0 {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/address-group/entry[@name='%s']", devicegroup[0], name)
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

// CreateSharedStaticGroup will create a new shared static address group on Panorama. You can specify multiple members
// by separating them with a comma, i.e. "web-server1, web-server2".
func (p *PaloAlto) CreateSharedStaticGroup(name, members, description string) error {
	var xmlBody string
	var xpath string
	var reqError requestError
	r := rested.NewRequest()
	m := strings.Split(members, ",")

	if members == "" {
		return errors.New("you cannot create a static address group without any members")
	}

	xmlBody = "<static>"
	for _, member := range m {
		xmlBody += fmt.Sprintf("<member>%s</member>", strings.TrimSpace(member))
	}
	xmlBody += "</static>"

	if description != "" {
		xmlBody += fmt.Sprintf("<description>%s</description>", description)
	}

	if p.DeviceType == "panos" {
		return errors.New("you can only create shared objects when connected to a Panorama device")
	}

	if p.DeviceType == "panorama" {
		xpath = fmt.Sprintf("/config/shared/address-group/entry[@name='%s']", name)
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

// CreateDynamicGroup will create a new dynamic address group on the device. The filter must be written like so:
// 'vm-servers' and 'some tag' or 'pcs' - using the tags as the match criteria. If creating an address group on a
// Panorama device, then specify the given device-group name as the last parameter.
func (p *PaloAlto) CreateDynamicGroup(name, criteria, description string, devicegroup ...string) error {
	xmlBody := fmt.Sprintf("<dynamic><filter>%s</filter></dynamic>", criteria)
	var xpath string
	var reqError requestError
	r := rested.NewRequest()

	if criteria == "" {
		return errors.New("you cannot create a dynamic address group without any filter")
	}

	if description != "" {
		xmlBody += fmt.Sprintf("<description>%s</description>", description)
	}

	if p.DeviceType == "panos" {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/address-group/entry[@name='%s']", name)
	}

	if p.DeviceType == "panorama" && len(devicegroup) > 0 {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/address-group/entry[@name='%s']", devicegroup[0], name)
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

// CreateSharedDynamicGroup will create a new shared dynamic address group on Panorama. The filter must be written like so:
// 'vm-servers' and 'some tag' or 'pcs' - using the tags as the match criteria.
func (p *PaloAlto) CreateSharedDynamicGroup(name, criteria, description string) error {
	xmlBody := fmt.Sprintf("<dynamic><filter>%s</filter></dynamic>", criteria)
	var xpath string
	var reqError requestError
	r := rested.NewRequest()

	if criteria == "" {
		return errors.New("you cannot create a dynamic address group without any filter")
	}

	if description != "" {
		xmlBody += fmt.Sprintf("<description>%s</description>", description)
	}

	if p.DeviceType == "panos" {
		return errors.New("you can only create shared objects when connected to a Panorama device")
	}

	if p.DeviceType == "panorama" {
		xpath = fmt.Sprintf("/config/shared/address-group/entry[@name='%s']", name)
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

// DeleteAddress will remove an address object from the device. If deleting an address object on a
// Panorama device, then specify the given device-group name as the last parameter.
func (p *PaloAlto) DeleteAddress(name string, devicegroup ...string) error {
	var xpath string
	var reqError requestError
	r := rested.NewRequest()

	if p.DeviceType == "panos" {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/address/entry[@name='%s']", name)
	}

	if p.DeviceType == "panorama" && len(devicegroup) > 0 {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/address/entry[@name='%s']", devicegroup[0], name)
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

// DeleteSharedAddress will remove a shared address object from Panorama.
func (p *PaloAlto) DeleteSharedAddress(name string) error {
	var xpath string
	var reqError requestError
	r := rested.NewRequest()

	if p.DeviceType == "panos" {
		return errors.New("you can only remove shared objects when connected to a Panorama device")
	}

	if p.DeviceType == "panorama" {
		xpath = fmt.Sprintf("/config/shared/address/entry[@name='%s']", name)
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

// DeleteAddressGroup will remove an address group from the device. If deleting an address group on a
// Panorama device, then specify the given device-group name as the last parameter.
func (p *PaloAlto) DeleteAddressGroup(name string, devicegroup ...string) error {
	var xpath string
	var reqError requestError
	r := rested.NewRequest()

	if p.DeviceType == "panos" {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='vsys1']/address-group/entry[@name='%s']", name)
	}

	if p.DeviceType == "panorama" && len(devicegroup) > 0 {
		xpath = fmt.Sprintf("/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='%s']/address-group/entry[@name='%s']", devicegroup[0], name)
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

// DeleteSharedAddressGroup will remove a shared address group from Panorama.
func (p *PaloAlto) DeleteSharedAddressGroup(name string) error {
	var xpath string
	var reqError requestError
	r := rested.NewRequest()

	if p.DeviceType == "panos" {
		return errors.New("you can only create shared objects when connected to a Panorama device")
	}

	if p.DeviceType == "panorama" {
		xpath = fmt.Sprintf("/config/shared/address-group/entry[@name='%s']", name)
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
