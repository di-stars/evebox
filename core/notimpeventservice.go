/* Copyright (c) 2016 Jason Ish
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 *
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED ``AS IS'' AND ANY EXPRESS OR IMPLIED
 * WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
 * DISCLAIMED. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY DIRECT,
 * INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
 * (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
 * SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
 * STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING
 * IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
 * POSSIBILITY OF SUCH DAMAGE.
 */

package core

import "fmt"

var NotImplementedError error

func init() {
	NotImplementedError = fmt.Errorf("Not implemented.")
}

// NotImplementedEventService is an implementation of core.EventService with
// every function created but returning an not implemented error.
type NotImplementedEventService struct {
}

func (s *NotImplementedEventService) GetEventById(id string) (map[string]interface{}, error) {
	return nil, NotImplementedError
}

func (s *NotImplementedEventService) AddTagsToEvent(id string, tags []string) error {
	return NotImplementedError
}

func (s *NotImplementedEventService) AddTagsToAlertGroup(p AlertGroupQueryParams, tags []string) error {
	return NotImplementedError
}

func (s *NotImplementedEventService) RemoveTagsFromAlertGroup(p AlertGroupQueryParams, tags []string) error {
	return NotImplementedError
}

func (s *NotImplementedEventService) RemoveTagsFromEvent(id string, tags []string) error {
	return NotImplementedError
}

func (s *NotImplementedEventService) ArchiveAlertGroup(p AlertGroupQueryParams) error {
	return NotImplementedError
}

func (s *NotImplementedEventService) EscalateAlertGroup(p AlertGroupQueryParams) error {
	return NotImplementedError
}

func (s *NotImplementedEventService) FindNetflow(options EventQueryOptions, sortBy string, order string) (interface{}, error) {
	return nil, NotImplementedError
}
