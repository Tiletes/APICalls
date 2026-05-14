// Package wsdl provides utilities for parsing WSDL 1.1 documents and
// mapping their operations to API template fields.
package wsdl

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	nsSOAP11 = "http://schemas.xmlsoap.org/wsdl/soap/"
	nsSOAP12 = "http://schemas.xmlsoap.org/wsdl/soap12/"
)

// ── Public types ─────────────────────────────────────────────────────────────

// ParsedOperation represents one WSDL operation mapped to an API template.
type ParsedOperation struct {
	Name        string `json:"name"`
	ServiceName string `json:"service_name"`
	Method      string `json:"method"`
	URL         string `json:"url"`
	Body        string `json:"body"`
	Headers     []KV   `json:"headers"`
}

// KV is a key/value pair used for headers or custom values.
type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ── Schema type model ────────────────────────────────────────────────────────

// schemaField represents one named child of a complexType (element or attribute).
type schemaField struct {
	Name      string
	TypeName  string // local name of the type (may be in another namespace)
	TypeNS    string // target namespace of the type
	IsAttr    bool   // true when sourced from xs:attribute
	Ref       string // if non-empty, this is a ref= to another top-level element
	RefNS     string // namespace URI of the ref target
	MinOccurs string // "" means default (1); "0" means optional
	MaxOccurs string // "" means default (1); "unbounded" or a positive integer
}

// schemaType holds the resolved children of a complexType or top-level element.
type schemaType struct {
	Fields []schemaField
}

// schemaModel accumulates all type/element definitions across all xs:schema blocks.
type schemaModel struct {
	// elements: targetNS → localName → schemaType (immediate children)
	elements map[string]map[string]*schemaType
	// complexTypes: targetNS → localName → schemaType
	complexTypes map[string]map[string]*schemaType
}

func newSchemaModel() *schemaModel {
	return &schemaModel{
		elements:     make(map[string]map[string]*schemaType),
		complexTypes: make(map[string]map[string]*schemaType),
	}
}

func (s *schemaModel) addElement(ns, name string, t *schemaType) {
	if ns == "" {
		ns = "_"
	}
	if s.elements[ns] == nil {
		s.elements[ns] = make(map[string]*schemaType)
	}
	s.elements[ns][name] = t
}

func (s *schemaModel) addComplexType(ns, name string, t *schemaType) {
	if ns == "" {
		ns = "_"
	}
	if s.complexTypes[ns] == nil {
		s.complexTypes[ns] = make(map[string]*schemaType)
	}
	s.complexTypes[ns][name] = t
}

// resolveElement looks up a top-level element, first by namespace then globally.
func (s *schemaModel) resolveElement(ns, name string) *schemaType {
	if ns != "" {
		if m, ok := s.elements[ns]; ok {
			if t, ok2 := m[name]; ok2 {
				return t
			}
		}
	}
	for _, m := range s.elements {
		if t, ok := m[name]; ok {
			return t
		}
	}
	return nil
}

// resolveComplexType looks up a complexType by local name and target namespace.
func (s *schemaModel) resolveComplexType(ns, name string) *schemaType {
	if ns != "" {
		if m, ok := s.complexTypes[ns]; ok {
			if t, ok2 := m[name]; ok2 {
				return t
			}
		}
	}
	for _, m := range s.complexTypes {
		if t, ok := m[name]; ok {
			return t
		}
	}
	return nil
}

// ── WSDL model ───────────────────────────────────────────────────────────────

type wsdlModel struct {
	TargetNS    string
	ServiceName string
	SoapVersion string // "1.1" or "1.2"

	// Messages: local message name → part element local name
	Messages map[string]string
	// MessageNS: local message name → namespace URI of the element
	MessageNS map[string]string

	// BindingOps: binding local name → op name → soapAction
	BindingOps map[string]map[string]string
	// BindingTypes: binding local name → portType local name
	BindingTypes map[string]string
	// PortTypeOps: portType local name → op name → input message local name
	PortTypeOps map[string]map[string]string
	// Endpoints: binding local name → SOAP endpoint URL
	Endpoints map[string]string

	// BindingOpHeaderMsg: binding → op → header message local name (resolved after full parse)
	BindingOpHeaderMsg map[string]map[string]string
	// NSHints: namespace URI → preferred prefix (from root xmlns: declarations)
	NSHints map[string]string

	schema *schemaModel
}

type opEntry struct {
	Name           string
	SoapAction     string
	InputMsg       string
	Endpoint       string
	HeaderElemName string
	HeaderElemNS   string
}

// ── Public API ───────────────────────────────────────────────────────────────

// FetchFromURL downloads a WSDL from a remote URL and parses it.
// Only http and https schemes are accepted.
func FetchFromURL(rawURL string) ([]ParsedOperation, error) {
	c := &http.Client{Timeout: 20 * time.Second}
	resp, err := c.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetch WSDL: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return nil, fmt.Errorf("read WSDL body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if fault := extractSOAPFault(data); fault != "" {
			return nil, fmt.Errorf("server returned HTTP %d with SOAP fault: %s", resp.StatusCode, fault)
		}
		return nil, fmt.Errorf("server returned HTTP %d — not a valid WSDL response", resp.StatusCode)
	}
	return Parse(data)
}

// Parse parses raw WSDL bytes and returns the contained operations.
func Parse(data []byte) ([]ParsedOperation, error) {
	if err := checkIsWSDL(data); err != nil {
		return nil, err
	}
	m := &wsdlModel{
		Messages:           make(map[string]string),
		MessageNS:          make(map[string]string),
		BindingOps:         make(map[string]map[string]string),
		BindingTypes:       make(map[string]string),
		PortTypeOps:        make(map[string]map[string]string),
		Endpoints:          make(map[string]string),
		BindingOpHeaderMsg: make(map[string]map[string]string),
		NSHints:            make(map[string]string),
		schema:             newSchemaModel(),
	}
	if err := parseSchema(data, m.schema); err != nil {
		return nil, err
	}
	if err := parseWSDL(data, m); err != nil {
		return nil, err
	}
	return buildOperations(m), nil
}

// ── Schema parser ─────────────────────────────────────────────────────────────

// parseSchema does a DOM-style walk of all xs:schema / xsd:schema blocks
// inside <wsdl:types>.  It builds a full map of top-level elements and
// complexTypes, recording both xs:element children and xs:attribute children.

func parseSchema(data []byte, s *schemaModel) error {
	type xmlNode struct {
		XMLName  xml.Name
		Attrs    []xml.Attr `xml:",any,attr"`
		Children []*xmlNode `xml:",any"`
	}
	var root xmlNode
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	if err := dec.Decode(&root); err != nil {
		return fmt.Errorf("wsdl xml decode: %w", err)
	}

	nodeAttr := func(n *xmlNode, local string) string {
		for _, a := range n.Attrs {
			if a.Name.Local == local {
				return a.Value
			}
		}
		return ""
	}

	// Build the namespace map for a given node (xmlns:prefix="uri" attributes).
	nsMapOf := func(n *xmlNode) map[string]string {
		m := make(map[string]string)
		for _, a := range n.Attrs {
			if a.Name.Space == "xmlns" {
				m[a.Name.Local] = a.Value
			} else if a.Name.Local == "xmlns" && a.Name.Space == "" {
				m[""] = a.Value
			}
		}
		return m
	}

	// resolveQName splits "prefix:local" using a namespace map.
	resolveQName := func(qname string, nss map[string]string) (uri, local string) {
		if i := strings.Index(qname, ":"); i >= 0 {
			return nss[qname[:i]], qname[i+1:]
		}
		return nss[""], qname
	}

	isSchemaSpace := func(space string) bool {
		return strings.Contains(space, "XMLSchema") || strings.Contains(space, "xmlschema")
	}

	// walkComplexType extracts fields (elements + attributes) from a complexType node.
	var walkComplexType func(n *xmlNode, tgtNS string, nss map[string]string) *schemaType
	walkComplexType = func(n *xmlNode, tgtNS string, nss map[string]string) *schemaType {
		t := &schemaType{}
		for _, child := range n.Children {
			if !isSchemaSpace(child.XMLName.Space) {
				continue
			}
			switch child.XMLName.Local {
			case "sequence", "all", "choice", "complexContent", "simpleContent",
				"extension", "restriction", "group":
				inner := walkComplexType(child, tgtNS, nss)
				t.Fields = append(t.Fields, inner.Fields...)

			case "element":
				name := nodeAttr(child, "name")
				ref := nodeAttr(child, "ref")
				typeName := nodeAttr(child, "type")
				minOccurs := nodeAttr(child, "minOccurs")
				maxOccurs := nodeAttr(child, "maxOccurs")
				if name != "" {
					typeNS, typeLocal := resolveQName(typeName, nss)
					t.Fields = append(t.Fields, schemaField{
						Name:      name,
						TypeName:  typeLocal,
						TypeNS:    typeNS,
						MinOccurs: minOccurs,
						MaxOccurs: maxOccurs,
					})
				} else if ref != "" {
					refNS, refLocal := resolveQName(ref, nss)
					t.Fields = append(t.Fields, schemaField{
						Name:      refLocal,
						Ref:       refLocal,
						RefNS:     refNS,
						MinOccurs: minOccurs,
						MaxOccurs: maxOccurs,
					})
				}

			case "attribute":
				name := nodeAttr(child, "name")
				typeName := nodeAttr(child, "type")
				if name != "" {
					typeNS, typeLocal := resolveQName(typeName, nss)
					t.Fields = append(t.Fields, schemaField{
						Name:     name,
						TypeName: typeLocal,
						TypeNS:   typeNS,
						IsAttr:   true,
					})
				}
			}
		}
		return t
	}

	// walkSchema processes one xs:schema node.
	walkSchema := func(schemaNode *xmlNode) {
		tgtNS := nodeAttr(schemaNode, "targetNamespace")
		// Merge namespace declarations with a default tns→tgtNS mapping.
		nss := nsMapOf(schemaNode)
		if tgtNS != "" {
			if _, ok := nss["tns"]; !ok {
				nss["tns"] = tgtNS
			}
		}

		for _, child := range schemaNode.Children {
			if !isSchemaSpace(child.XMLName.Space) {
				continue
			}
			switch child.XMLName.Local {
			case "element":
				name := nodeAttr(child, "name")
				if name == "" {
					continue
				}
				t := &schemaType{}
				// Inline complexType wins over type= attribute.
				for _, gc := range child.Children {
					if gc.XMLName.Local == "complexType" && isSchemaSpace(gc.XMLName.Space) {
						t = walkComplexType(gc, tgtNS, nss)
					}
				}
				if typeName := nodeAttr(child, "type"); typeName != "" && len(t.Fields) == 0 {
					typeNS, typeLocal := resolveQName(typeName, nss)
					// Store as a single sentinel field so we can resolve it in writeElement.
					t.Fields = []schemaField{{
						Name:     "_type",
						TypeName: typeLocal,
						TypeNS:   typeNS,
					}}
				}
				s.addElement(tgtNS, name, t)

			case "complexType":
				name := nodeAttr(child, "name")
				if name == "" {
					continue
				}
				t := walkComplexType(child, tgtNS, nss)
				s.addComplexType(tgtNS, name, t)
			}
		}
	}

	// Locate wsdl:types and iterate its schema children.
	var visitNode func(n *xmlNode)
	visitNode = func(n *xmlNode) {
		if n.XMLName.Local == "types" {
			for _, child := range n.Children {
				if child.XMLName.Local == "schema" {
					walkSchema(child)
				}
			}
			return
		}
		for _, child := range n.Children {
			visitNode(child)
		}
	}
	visitNode(&root)
	return nil
}

// ── WSDL parser (streaming) ───────────────────────────────────────────────────

func parseWSDL(data []byte, m *wsdlModel) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false

	type frame struct{ space, local string }
	var stack []frame
	push := func(space, local string) { stack = append(stack, frame{space, local}) }
	pop := func() {
		if len(stack) > 0 {
			stack = stack[:len(stack)-1]
		}
	}
	depth := func() int { return len(stack) }
	hasAncestor := func(local string) bool {
		for _, f := range stack {
			if f.local == local {
				return true
			}
		}
		return false
	}

	// Maintain a per-element namespace scope stack so we can resolve qualified
	// attribute values (e.g. message="tns:input", binding="tns:Binding").
	type nsScope struct{ m map[string]string }
	var nsStack []nsScope
	pushNS := func(attrs []xml.Attr) {
		cur := make(map[string]string)
		if len(nsStack) > 0 {
			for k, v := range nsStack[len(nsStack)-1].m {
				cur[k] = v
			}
		}
		for _, a := range attrs {
			if a.Name.Space == "xmlns" {
				cur[a.Name.Local] = a.Value
			} else if a.Name.Local == "xmlns" && a.Name.Space == "" {
				cur[""] = a.Value
			}
		}
		nsStack = append(nsStack, nsScope{cur})
	}
	popNS := func() {
		if len(nsStack) > 0 {
			nsStack = nsStack[:len(nsStack)-1]
		}
	}
	resolveQName := func(qname string) (uri, local string) {
		if qname == "" {
			return "", ""
		}
		var nss map[string]string
		if len(nsStack) > 0 {
			nss = nsStack[len(nsStack)-1].m
		}
		if i := strings.Index(qname, ":"); i >= 0 {
			prefix := qname[:i]
			local := qname[i+1:]
			if nss != nil {
				return nss[prefix], local
			}
			return "", local
		}
		if nss != nil {
			return nss[""], qname
		}
		return "", qname
	}

	var (
		curMsg          string
		curPortType     string
		curPTOp         string
		curBinding      string
		curBindingDepth int
		curBindOp       string
		curBindOpDepth  int
		curService      string
		curPortBinding  string
		inBindOpInput   bool
	)

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("wsdl xml: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			pushNS(t.Attr)
			push(t.Name.Space, t.Name.Local)
			local := t.Name.Local
			space := t.Name.Space
			d := depth()
			av := func(name string) string { return attrVal(t.Attr, name) }

			switch local {
			case "definitions":
				if d == 1 {
					m.TargetNS = av("targetNamespace")
					if n := av("name"); n != "" {
						m.ServiceName = n
					}
					// Capture namespace prefix hints for readable XML output.
					if len(nsStack) > 0 {
						for pfx, uri := range nsStack[len(nsStack)-1].m {
							if pfx != "" && uri != "" {
								m.NSHints[uri] = pfx
							}
						}
					}
				}

			case "message":
				if d == 2 {
					curMsg = av("name")
				}

			case "part":
				if curMsg != "" {
					if elemAttr := av("element"); elemAttr != "" {
						ns, loc := resolveQName(elemAttr)
						m.Messages[curMsg] = loc
						m.MessageNS[curMsg] = ns
					}
				}

			case "portType":
				if d == 2 {
					curPortType = av("name")
					if m.PortTypeOps[curPortType] == nil {
						m.PortTypeOps[curPortType] = make(map[string]string)
					}
				}

			case "operation":
				switch {
				case curPortType != "" && !hasAncestor("binding") && d == 3:
					curPTOp = av("name")

				case curBinding != "" && !isSOAPNS(space) && d == curBindingDepth+1:
					curBindOp = av("name")
					curBindOpDepth = d
					if m.BindingOps[curBinding] == nil {
						m.BindingOps[curBinding] = make(map[string]string)
					}
					if curBindOp != "" {
						m.BindingOps[curBinding][curBindOp] = ""
					}

				case curBindOp != "" && isSOAPNS(space):
					sa := av("soapAction")
					if m.BindingOps[curBinding] == nil {
						m.BindingOps[curBinding] = make(map[string]string)
					}
					m.BindingOps[curBinding][curBindOp] = sa
					if m.SoapVersion == "" {
						if space == nsSOAP12 {
							m.SoapVersion = "1.2"
						} else {
							m.SoapVersion = "1.1"
						}
					}
				}

			case "input":
				if curPTOp != "" && hasAncestor("portType") {
					if msg := av("message"); msg != "" {
						_, msgLocal := resolveQName(msg)
						m.PortTypeOps[curPortType][curPTOp] = msgLocal
					}
				}
				if curBindOp != "" && hasAncestor("binding") {
					inBindOpInput = true
				}

			case "header":
				// <soap:header> inside a binding operation's input — store the message name for
				// later resolution (the <message> element may appear after <binding> in the WSDL).
				if inBindOpInput && isSOAPNS(space) {
					if msgQName := av("message"); msgQName != "" {
						_, msgLocal := resolveQName(msgQName)
						if msgLocal != "" {
							if m.BindingOpHeaderMsg[curBinding] == nil {
								m.BindingOpHeaderMsg[curBinding] = make(map[string]string)
							}
							// Only store the first header found for this operation.
							if m.BindingOpHeaderMsg[curBinding][curBindOp] == "" {
								m.BindingOpHeaderMsg[curBinding][curBindOp] = msgLocal
							}
						}
					}
				}

			case "binding":
				if d == 2 {
					curBinding = av("name")
					curBindingDepth = d
					if tp := av("type"); tp != "" {
						_, tpLocal := resolveQName(tp)
						m.BindingTypes[curBinding] = tpLocal
					}
				}

			case "service":
				if d == 2 {
					curService = av("name")
					m.ServiceName = curService
				}

			case "port":
				if curService != "" {
					_, bindLocal := resolveQName(av("binding"))
					curPortBinding = bindLocal
				}

			case "address":
				if curPortBinding != "" && isSOAPNS(space) {
					if loc := av("location"); loc != "" {
						m.Endpoints[curPortBinding] = loc
					}
				}
			}

		case xml.EndElement:
			local := t.Name.Local
			d := depth()

			switch local {
			case "message":
				if d == 2 {
					curMsg = ""
				}
			case "input":
				inBindOpInput = false
			case "portType":
				if d == 2 {
					curPortType = ""
					curPTOp = ""
				}
			case "operation":
				if curPortType != "" && d == 3 {
					curPTOp = ""
				}
				if curBinding != "" && d == curBindOpDepth {
					curBindOp = ""
					curBindOpDepth = 0
				}
			case "binding":
				if d == 2 {
					curBinding = ""
					curBindingDepth = 0
					curBindOp = ""
				}
			case "service":
				if d == 2 {
					curService = ""
				}
			case "port":
				curPortBinding = ""
			}

			pop()
			popNS()
		}
	}
	return nil
}

// ── Operation assembly ───────────────────────────────────────────────────────

func buildOperations(m *wsdlModel) []ParsedOperation {
	var entries []opEntry

	for bindingName, ops := range m.BindingOps {
		ptName := m.BindingTypes[bindingName]
		endpoint := m.Endpoints[bindingName]
		ptOps := m.PortTypeOps[ptName]

		for opName, soapAction := range ops {
			inputMsg := ""
			if ptOps != nil {
				inputMsg = ptOps[opName]
			}
			if inputMsg == "" {
				for _, ptMap := range m.PortTypeOps {
					if msg, ok := ptMap[opName]; ok {
						inputMsg = msg
						break
					}
				}
			}
			headerElemName := ""
			headerElemNS := ""
			if hdrs, ok := m.BindingOpHeaderMsg[bindingName]; ok {
				if hdrMsgName := hdrs[opName]; hdrMsgName != "" {
					// Resolve now — the full message map is available after parseWSDL returns.
					headerElemName = m.Messages[hdrMsgName]
					headerElemNS = m.MessageNS[hdrMsgName]
				}
			}
			entries = append(entries, opEntry{
				Name:           opName,
				SoapAction:     soapAction,
				InputMsg:       inputMsg,
				Endpoint:       endpoint,
				HeaderElemName: headerElemName,
				HeaderElemNS:   headerElemNS,
			})
		}
	}

	// Fallback: abstract WSDLs with portTypes but no SOAP binding.
	if len(entries) == 0 {
		endpoint := ""
		for _, ep := range m.Endpoints {
			endpoint = ep
			break
		}
		for _, ptOps := range m.PortTypeOps {
			for opName, inputMsg := range ptOps {
				entries = append(entries, opEntry{
					Name:     opName,
					InputMsg: inputMsg,
					Endpoint: endpoint,
				})
			}
		}
	}

	// Deduplicate by (operation name + soapAction) so that multiple bindings
	// for the same logical operation (e.g. DMZ vs LAN) each produce a
	// separate selectable entry, while true duplicates are still collapsed.
	seen := make(map[string]opEntry)
	for _, e := range entries {
		key := e.Name + "\x00" + e.SoapAction
		if _, ok := seen[key]; !ok {
			seen[key] = e
		}
	}

	result := make([]ParsedOperation, 0, len(seen))
	for _, e := range seen {
		result = append(result, assembleOperation(m, e))
	}
	return result
}

func assembleOperation(m *wsdlModel, e opEntry) ParsedOperation {
	soapVersion := m.SoapVersion
	if soapVersion == "" {
		soapVersion = "1.1"
	}

	rootElemName := m.Messages[e.InputMsg]
	rootElemNS := m.MessageNS[e.InputMsg]
	if rootElemName == "" {
		rootElemName = e.Name
	}

	body := buildSOAPBody(m, rootElemName, rootElemNS, e.HeaderElemName, e.HeaderElemNS, soapVersion)
	headers := buildSOAPHeaders(e.SoapAction, soapVersion)

	serviceName := m.ServiceName
	if stripped := strings.Trim(e.SoapAction, `"`); stripped != "" {
		serviceName = stripped
	}

	return ParsedOperation{
		Name:        e.Name,
		ServiceName: serviceName,
		Method:      "POST",
		URL:         e.Endpoint,
		Body:        body,
		Headers:     headers,
	}
}

// ── SOAP body builder ─────────────────────────────────────────────────────────

const maxExpansionDepth = 6

func buildSOAPBody(m *wsdlModel, rootElemName, rootElemNS, headerElemName, headerElemNS, soapVersion string) string {
	envNS := "http://schemas.xmlsoap.org/soap/envelope/"
	envPfx := "soapenv"
	if soapVersion == "1.2" {
		envNS = "http://www.w3.org/2003/05/soap-envelope"
		envPfx = "soap"
	}

	nsReg := &nsRegistry{
		prefixes: make(map[string]string),
		hints:    m.NSHints,
	}
	if rootElemNS != "" {
		nsReg.prefix(rootElemNS)
	}

	// Build body content first so all namespaces are registered before the Envelope tag.
	var bodyBuf strings.Builder
	writeElement(&bodyBuf, m, nsReg, rootElemName, rootElemNS, 2, make(map[string]int))

	// Build header content if a SOAP header element was found in the binding.
	var headerBuf strings.Builder
	if headerElemName != "" {
		if headerElemNS != "" {
			nsReg.prefix(headerElemNS)
		}
		writeElement(&headerBuf, m, nsReg, headerElemName, headerElemNS, 2, make(map[string]int))
	}

	// Collect all xmlns declarations gathered during body + header expansion.
	var nsParts strings.Builder
	for uri, pfx := range nsReg.prefixes {
		nsParts.WriteString(fmt.Sprintf(" xmlns:%s=%q", pfx, uri))
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(`<%s:Envelope xmlns:%s="%s"%s>`, envPfx, envPfx, envNS, nsParts.String()))
	if headerElemName != "" {
		sb.WriteString(fmt.Sprintf("\n  <%s:Header>", envPfx))
		sb.WriteString(headerBuf.String())
		sb.WriteString(fmt.Sprintf("\n  </%s:Header>", envPfx))
	} else {
		sb.WriteString(fmt.Sprintf("\n  <%s:Header/>", envPfx))
	}
	sb.WriteString(fmt.Sprintf("\n  <%s:Body>", envPfx))
	sb.WriteString(bodyBuf.String())
	sb.WriteString(fmt.Sprintf("\n  </%s:Body>", envPfx))
	sb.WriteString(fmt.Sprintf("\n</%s:Envelope>", envPfx))
	return sb.String()
}

type nsRegistry struct {
	prefixes map[string]string // uri → short prefix
	hints    map[string]string // uri → preferred prefix (from WSDL xmlns: declarations)
	counter  int
}

func (r *nsRegistry) prefix(uri string) string {
	if p, ok := r.prefixes[uri]; ok {
		return p
	}
	// Use the hint prefix when available and not already taken by another URI.
	if r.hints != nil {
		if hint, ok := r.hints[uri]; ok && !r.isPrefixUsed(hint) {
			r.prefixes[uri] = hint
			return hint
		}
	}
	p := fmt.Sprintf("tns%d", r.counter)
	r.counter++
	r.prefixes[uri] = p
	return p
}

func (r *nsRegistry) isPrefixUsed(pfx string) bool {
	for _, p := range r.prefixes {
		if p == pfx {
			return true
		}
	}
	return false
}

// writeElement writes one element (and its children) into buf at the given indent depth.
// seen tracks how many times each type name has been expanded to prevent infinite loops.
func writeElement(buf *strings.Builder, m *wsdlModel, nsReg *nsRegistry, name, ns string, indent int, seen map[string]int) {
	if indent > maxExpansionDepth*2 {
		return
	}
	pad := strings.Repeat("  ", indent)

	pfx := ""
	if ns != "" {
		pfx = nsReg.prefix(ns) + ":"
	}

	et := m.schema.resolveElement(ns, name)
	if et == nil {
		buf.WriteString(fmt.Sprintf("\n%s<%s%s>?</%s%s>", pad, pfx, name, pfx, name))
		return
	}

	// If the element was stored with a _type sentinel, expand via the complexType.
	fields := expandType(m, et, seen, 0)

	var attrs []schemaField
	var children []schemaField
	for _, f := range fields {
		if f.IsAttr {
			attrs = append(attrs, f)
		} else {
			children = append(children, f)
		}
	}

	var attrStr strings.Builder
	for _, a := range attrs {
		attrStr.WriteString(fmt.Sprintf(` %s="?"`, a.Name))
	}

	if len(children) == 0 {
		if len(attrs) == 0 {
			buf.WriteString(fmt.Sprintf("\n%s<%s%s>?</%s%s>", pad, pfx, name, pfx, name))
		} else {
			buf.WriteString(fmt.Sprintf("\n%s<%s%s%s/>", pad, pfx, name, attrStr.String()))
		}
		return
	}

	buf.WriteString(fmt.Sprintf("\n%s<%s%s%s>", pad, pfx, name, attrStr.String()))
	childPad := strings.Repeat("  ", indent+1)
	for _, child := range children {
		childSeen := copySeenMap(seen)
		if comment := occurrenceComment(child.MinOccurs, child.MaxOccurs); comment != "" {
			buf.WriteString(fmt.Sprintf("\n%s<!--%s-->", childPad, comment))
		}
		if child.Ref != "" {
			writeElement(buf, m, nsReg, child.Ref, child.RefNS, indent+1, childSeen)
		} else {
			writeElement(buf, m, nsReg, child.Name, child.TypeNS, indent+1, childSeen)
		}
	}
	buf.WriteString(fmt.Sprintf("\n%s</%s%s>", pad, pfx, name))
}

// expandType resolves a schemaType's fields: if a field has a TypeName that
// refers to a known complexType, it replaces that field's TypeNS with the
// type's namespace (for writeElement to use) and keeps it as a child node.
// The _type sentinel is handled here: if et has a single _type field, we
// inline the referenced complexType's fields directly (for top-level elements
// that are declared as type="SomeComplexType").
func expandType(m *wsdlModel, et *schemaType, seen map[string]int, depth int) []schemaField {
	if depth > maxExpansionDepth {
		return nil
	}

	// If this element was stored with a _type sentinel, inline the complex type.
	if len(et.Fields) == 1 && et.Fields[0].Name == "_type" {
		typeName := et.Fields[0].TypeName
		typeNS := et.Fields[0].TypeNS
		if seen[typeName] < 2 {
			ct := m.schema.resolveComplexType(typeNS, typeName)
			if ct != nil {
				newSeen := copySeenMap(seen)
				newSeen[typeName]++
				return expandType(m, ct, newSeen, depth+1)
			}
		}
		return nil
	}

	return et.Fields
}

// occurrenceComment returns the SOAP UI–style cardinality comment for a field,
// or an empty string when no comment is needed (required, single-occurrence).
func occurrenceComment(minOccurs, maxOccurs string) string {
	isUnbounded := maxOccurs == "unbounded" ||
		(maxOccurs != "" && maxOccurs != "0" && maxOccurs != "1")
	switch {
	case minOccurs == "0" && isUnbounded:
		return "Zero or more repetitions:"
	case (minOccurs == "" || minOccurs == "1") && isUnbounded:
		return "1 or more repetitions:"
	case minOccurs == "0":
		return "Optional:"
	default:
		return ""
	}
}

func copySeenMap(m map[string]int) map[string]int {
	c := make(map[string]int, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// ── SOAP header builder ───────────────────────────────────────────────────────

func buildSOAPHeaders(soapAction, soapVersion string) []KV {
	if soapVersion == "1.2" {
		ct := `application/soap+xml; charset=utf-8`
		if soapAction != "" {
			ct += `; action="` + soapAction + `"`
		}
		return []KV{{Key: "Content-Type", Value: ct}}
	}
	headers := []KV{{Key: "Content-Type", Value: "text/xml; charset=utf-8"}}
	headers = append(headers, KV{Key: "SOAPAction", Value: `"` + soapAction + `"`})
	return headers
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func attrVal(attrs []xml.Attr, local string) string {
	for _, a := range attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

func isSOAPNS(space string) bool {
	return space == nsSOAP11 || space == nsSOAP12
}

// checkIsWSDL inspects the first XML start element and returns an error if it
// is not a <definitions> root, surfacing SOAP Faults with their message text.
func checkIsWSDL(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	for {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("not a valid XML document: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		local := se.Name.Local
		if local == "definitions" {
			return nil
		}
		if local == "Envelope" || local == "Fault" {
			return fmt.Errorf("server returned a SOAP Fault instead of a WSDL: %s", extractSOAPFault(data))
		}
		return fmt.Errorf("unexpected root element <%s>: not a valid WSDL document", local)
	}
}

// extractSOAPFault scans data for a <faultstring> element and returns its text.
func extractSOAPFault(data []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	inFault := false
	for {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "faultstring" || t.Name.Local == "Text" {
				inFault = true
			}
		case xml.EndElement:
			inFault = false
		case xml.CharData:
			if inFault {
				if s := strings.TrimSpace(string(t)); s != "" {
					return s
				}
			}
		}
	}
}
