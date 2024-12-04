package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type IbnInput struct {
	PageNumber int `json:"page-number"`
	PageSize   int `json:"page-size"`
}

type IntentTypeSearchPayload struct {
	Input IbnInput `json:"ibn-administration:input"`
}

type IntentType struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

type IntentTypeList struct {
	Output struct {
		PageSize   int          `json:"page-size"`
		TotalCount int          `json:"total-count"`
		IntentType []IntentType `json:"intent-type"`
	} `json:"ibn-administration:output"`
}

type IntentTypeYangModule struct {
	Name        string `json:"name"`
	YangContent string `json:"yang-content"`
}

type IntentTypeDefinition struct {
	IntentType struct {
		Module []IntentTypeYangModule `json:"module"`
	} `json:"ibn-administration:intent-type"`
}

// Get available NSP intent types with pagination
func (s *srv) intentTypeSearch(pageNumber, pageSize int) ([]string, error) {
	payload := IntentTypeSearchPayload{
		Input: IbnInput{
			PageNumber: pageNumber,
			PageSize:   pageSize,
		},
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("[Error] generating intent type search payload: %s", err)
	}

	url := fmt.Sprintf("https://%s/restconf/operations/ibn-administration:search-intent-types", s.nsp.Ip)
	resp, err := s.makeHTTPRequest("POST", url, bytes.NewReader(reqBody), nil)
	if err != nil {
		return nil, fmt.Errorf("[Error] fetching intent types: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("[Error] fetching intent types, status: %d", resp.StatusCode)
	}

	var intentTypeList IntentTypeList
	if err := json.NewDecoder(resp.Body).Decode(&intentTypeList); err != nil {
		return nil, fmt.Errorf("[Error] decoding intent type search response: %s", err)
	}

	var intentTypes []string
	for _, intentType := range intentTypeList.Output.IntentType {
		intentTypes = append(intentTypes, fmt.Sprintf("%s_%d", intentType.Name, intentType.Version))
	}

	// Handle pagination
	if intentTypeList.Output.PageSize == pageSize && intentTypeList.Output.TotalCount > pageSize {
		nextPage, err := s.intentTypeSearch(pageNumber+1, pageSize)
		if err != nil {
			return nil, fmt.Errorf("[Error] fetching paginated intent types: %s", err)
		}
		intentTypes = append(intentTypes, nextPage...)
	}

	return intentTypes, nil
}

// Get YANG modules for a specific NSP intent type
func (s *srv) intentTypeYangModules(intentType string) ([]IntentTypeYangModule, error) {
	lastInd := strings.LastIndex(intentType, "_")
	if lastInd == -1 {
		return nil, fmt.Errorf("[Error] invalid intent type format: %s", intentType)
	}

	name := intentType[:lastInd]
	version := intentType[lastInd+1:]

	url := fmt.Sprintf(
		"https://%s/restconf/data/ibn-administration:ibn-administration/intent-type-catalog/intent-type=%s,%s",
		s.nsp.Ip, url.QueryEscape(name), version,
	)

	resp, err := s.makeHTTPRequest("GET", url, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("[Error] fetching YANG modules for intent type: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("[Error] fetching YANG modules, status: %d", resp.StatusCode)
	}

	var yangDef IntentTypeDefinition
	if err := json.NewDecoder(resp.Body).Decode(&yangDef); err != nil {
		return nil, fmt.Errorf("[Error] decoding YANG modules response: %s", err)
	}

	return yangDef.IntentType.Module, nil
}
