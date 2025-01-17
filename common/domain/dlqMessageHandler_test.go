// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package domain

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common/log/loggerimpl"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

type (
	dlqMessageHandlerSuite struct {
		suite.Suite

		*require.Assertions
		controller *gomock.Controller

		mockReplicationTaskExecutor *MockReplicationTaskExecutor
		mockReplicationQueue        *MockReplicationQueue
		dlqMessageHandler           *dlqMessageHandlerImpl
	}
)

func TestDLQMessageHandlerSuite(t *testing.T) {
	s := new(dlqMessageHandlerSuite)
	suite.Run(t, s)
}

func (s *dlqMessageHandlerSuite) SetupSuite() {
}

func (s *dlqMessageHandlerSuite) TearDownSuite() {

}

func (s *dlqMessageHandlerSuite) SetupTest() {
	s.Assertions = require.New(s.T())
	s.controller = gomock.NewController(s.T())

	s.mockReplicationTaskExecutor = NewMockReplicationTaskExecutor(s.controller)
	s.mockReplicationQueue = NewMockReplicationQueue(s.controller)

	logger := loggerimpl.NewLoggerForTest(s.Suite)
	s.dlqMessageHandler = NewDLQMessageHandler(
		s.mockReplicationTaskExecutor,
		s.mockReplicationQueue,
		logger,
		metrics.NewNoopMetricsClient(),
	).(*dlqMessageHandlerImpl)
}

func (s *dlqMessageHandlerSuite) TearDownTest() {
}

func (s *dlqMessageHandlerSuite) TestReadMessages() {
	ackLevel := int64(10)
	lastMessageID := int64(20)
	pageSize := 100
	pageToken := []byte{}

	tasks := []*types.ReplicationTask{
		{
			TaskType:     types.ReplicationTaskTypeDomain.Ptr(),
			SourceTaskID: 1,
		},
	}
	s.mockReplicationQueue.EXPECT().GetDLQAckLevel(gomock.Any()).Return(ackLevel, nil).Times(1)
	s.mockReplicationQueue.EXPECT().GetMessagesFromDLQ(gomock.Any(), ackLevel, lastMessageID, pageSize, pageToken).
		Return(tasks, nil, nil).Times(1)

	resp, token, err := s.dlqMessageHandler.Read(context.Background(), lastMessageID, pageSize, pageToken)

	s.NoError(err)
	s.Equal(tasks, resp)
	s.Nil(token)
}

func (s *dlqMessageHandlerSuite) TestReadMessages_ThrowErrorOnGetDLQAckLevel() {
	lastMessageID := int64(20)
	pageSize := 100
	pageToken := []byte{}

	tasks := []*types.ReplicationTask{
		{
			TaskType:     types.ReplicationTaskTypeDomain.Ptr(),
			SourceTaskID: 1,
		},
	}
	testError := fmt.Errorf("test")
	s.mockReplicationQueue.EXPECT().GetDLQAckLevel(gomock.Any()).Return(int64(-1), testError).Times(1)
	s.mockReplicationQueue.EXPECT().GetMessagesFromDLQ(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(tasks, nil, nil).Times(0)

	_, _, err := s.dlqMessageHandler.Read(context.Background(), lastMessageID, pageSize, pageToken)

	s.Equal(testError, err)
}

func (s *dlqMessageHandlerSuite) TestReadMessages_ThrowErrorOnReadMessages() {
	ackLevel := int64(10)
	lastMessageID := int64(20)
	pageSize := 100
	pageToken := []byte{}

	testError := fmt.Errorf("test")
	s.mockReplicationQueue.EXPECT().GetDLQAckLevel(gomock.Any()).Return(ackLevel, nil).Times(1)
	s.mockReplicationQueue.EXPECT().GetMessagesFromDLQ(gomock.Any(), ackLevel, lastMessageID, pageSize, pageToken).
		Return(nil, nil, testError).Times(1)

	_, _, err := s.dlqMessageHandler.Read(context.Background(), lastMessageID, pageSize, pageToken)

	s.Equal(testError, err)
}

func (s *dlqMessageHandlerSuite) TestPurgeMessages() {
	ackLevel := int64(10)
	lastMessageID := int64(20)

	s.mockReplicationQueue.EXPECT().GetDLQAckLevel(gomock.Any()).Return(ackLevel, nil).Times(1)
	s.mockReplicationQueue.EXPECT().RangeDeleteMessagesFromDLQ(gomock.Any(), ackLevel, lastMessageID).Return(nil).Times(1)
	s.mockReplicationQueue.EXPECT().UpdateDLQAckLevel(gomock.Any(), lastMessageID).Return(nil).Times(1)
	err := s.dlqMessageHandler.Purge(context.Background(), lastMessageID)

	s.NoError(err)
}

func (s *dlqMessageHandlerSuite) TestPurgeMessages_ThrowErrorOnGetDLQAckLevel() {
	lastMessageID := int64(20)
	testError := fmt.Errorf("test")

	s.mockReplicationQueue.EXPECT().GetDLQAckLevel(gomock.Any()).Return(int64(-1), testError).Times(1)
	s.mockReplicationQueue.EXPECT().RangeDeleteMessagesFromDLQ(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(0)
	s.mockReplicationQueue.EXPECT().UpdateDLQAckLevel(gomock.Any(), gomock.Any()).Times(0)
	err := s.dlqMessageHandler.Purge(context.Background(), lastMessageID)

	s.Equal(testError, err)
}

func (s *dlqMessageHandlerSuite) TestPurgeMessages_ThrowErrorOnPurgeMessages() {
	ackLevel := int64(10)
	lastMessageID := int64(20)
	testError := fmt.Errorf("test")

	s.mockReplicationQueue.EXPECT().GetDLQAckLevel(gomock.Any()).Return(ackLevel, nil).Times(1)
	s.mockReplicationQueue.EXPECT().RangeDeleteMessagesFromDLQ(gomock.Any(), ackLevel, lastMessageID).Return(testError).Times(1)
	s.mockReplicationQueue.EXPECT().UpdateDLQAckLevel(gomock.Any(), gomock.Any()).Times(0)
	err := s.dlqMessageHandler.Purge(context.Background(), lastMessageID)

	s.Equal(testError, err)
}

func (s *dlqMessageHandlerSuite) TestMergeMessages() {
	ackLevel := int64(10)
	lastMessageID := int64(20)
	pageSize := 100
	pageToken := []byte{}
	messageID := int64(11)

	domainAttribute := &types.DomainTaskAttributes{
		ID: uuid.New(),
	}

	tasks := []*types.ReplicationTask{
		{
			TaskType:             types.ReplicationTaskTypeDomain.Ptr(),
			SourceTaskID:         messageID,
			DomainTaskAttributes: domainAttribute,
		},
	}
	s.mockReplicationQueue.EXPECT().GetDLQAckLevel(gomock.Any()).Return(ackLevel, nil).Times(1)
	s.mockReplicationQueue.EXPECT().GetMessagesFromDLQ(gomock.Any(), ackLevel, lastMessageID, pageSize, pageToken).
		Return(tasks, nil, nil).Times(1)
	s.mockReplicationTaskExecutor.EXPECT().Execute(domainAttribute).Return(nil).Times(1)
	s.mockReplicationQueue.EXPECT().UpdateDLQAckLevel(gomock.Any(), messageID).Return(nil).Times(1)
	s.mockReplicationQueue.EXPECT().RangeDeleteMessagesFromDLQ(gomock.Any(), ackLevel, messageID).Return(nil).Times(1)

	token, err := s.dlqMessageHandler.Merge(context.Background(), lastMessageID, pageSize, pageToken)
	s.NoError(err)
	s.Nil(token)
}

func (s *dlqMessageHandlerSuite) TestMergeMessages_ThrowErrorOnGetDLQAckLevel() {
	lastMessageID := int64(20)
	pageSize := 100
	pageToken := []byte{}
	messageID := int64(11)
	testError := fmt.Errorf("test")
	domainAttribute := &types.DomainTaskAttributes{
		ID: uuid.New(),
	}

	tasks := []*types.ReplicationTask{
		{
			TaskType:             types.ReplicationTaskTypeDomain.Ptr(),
			SourceTaskID:         messageID,
			DomainTaskAttributes: domainAttribute,
		},
	}
	s.mockReplicationQueue.EXPECT().GetDLQAckLevel(gomock.Any()).Return(int64(-1), testError).Times(1)
	s.mockReplicationQueue.EXPECT().GetMessagesFromDLQ(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(tasks, nil, nil).Times(0)
	s.mockReplicationTaskExecutor.EXPECT().Execute(gomock.Any()).Times(0)
	s.mockReplicationQueue.EXPECT().DeleteMessageFromDLQ(gomock.Any(), gomock.Any()).Times(0)
	s.mockReplicationQueue.EXPECT().UpdateDLQAckLevel(gomock.Any(), gomock.Any()).Times(0)

	token, err := s.dlqMessageHandler.Merge(context.Background(), lastMessageID, pageSize, pageToken)
	s.Equal(testError, err)
	s.Nil(token)
}

func (s *dlqMessageHandlerSuite) TestMergeMessages_ThrowErrorOnGetDLQMessages() {
	ackLevel := int64(10)
	lastMessageID := int64(20)
	pageSize := 100
	pageToken := []byte{}
	testError := fmt.Errorf("test")

	s.mockReplicationQueue.EXPECT().GetDLQAckLevel(gomock.Any()).Return(ackLevel, nil).Times(1)
	s.mockReplicationQueue.EXPECT().GetMessagesFromDLQ(gomock.Any(), ackLevel, lastMessageID, pageSize, pageToken).
		Return(nil, nil, testError).Times(1)
	s.mockReplicationTaskExecutor.EXPECT().Execute(gomock.Any()).Times(0)
	s.mockReplicationQueue.EXPECT().DeleteMessageFromDLQ(gomock.Any(), gomock.Any()).Times(0)
	s.mockReplicationQueue.EXPECT().UpdateDLQAckLevel(gomock.Any(), gomock.Any()).Times(0)

	token, err := s.dlqMessageHandler.Merge(context.Background(), lastMessageID, pageSize, pageToken)
	s.Equal(testError, err)
	s.Nil(token)
}

func (s *dlqMessageHandlerSuite) TestMergeMessages_ThrowErrorOnHandleReceivingTask() {
	ackLevel := int64(10)
	lastMessageID := int64(20)
	pageSize := 100
	pageToken := []byte{}
	messageID1 := int64(11)
	messageID2 := int64(12)
	testError := fmt.Errorf("test")
	domainAttribute1 := &types.DomainTaskAttributes{
		ID: uuid.New(),
	}
	domainAttribute2 := &types.DomainTaskAttributes{
		ID: uuid.New(),
	}
	tasks := []*types.ReplicationTask{
		{
			TaskType:             types.ReplicationTaskTypeDomain.Ptr(),
			SourceTaskID:         messageID1,
			DomainTaskAttributes: domainAttribute1,
		},
		{
			TaskType:             types.ReplicationTaskTypeDomain.Ptr(),
			SourceTaskID:         messageID2,
			DomainTaskAttributes: domainAttribute2,
		},
	}
	s.mockReplicationQueue.EXPECT().GetDLQAckLevel(gomock.Any()).Return(ackLevel, nil).Times(1)
	s.mockReplicationQueue.EXPECT().GetMessagesFromDLQ(gomock.Any(), ackLevel, lastMessageID, pageSize, pageToken).
		Return(tasks, nil, nil).Times(1)
	s.mockReplicationTaskExecutor.EXPECT().Execute(domainAttribute1).Return(nil).Times(1)
	s.mockReplicationTaskExecutor.EXPECT().Execute(domainAttribute2).Return(testError).Times(1)
	s.mockReplicationQueue.EXPECT().DeleteMessageFromDLQ(gomock.Any(), messageID1).Return(nil).Times(1)
	s.mockReplicationQueue.EXPECT().DeleteMessageFromDLQ(gomock.Any(), messageID2).Times(0)
	s.mockReplicationQueue.EXPECT().UpdateDLQAckLevel(gomock.Any(), messageID1).Return(nil).Times(1)

	token, err := s.dlqMessageHandler.Merge(context.Background(), lastMessageID, pageSize, pageToken)
	s.Equal(testError, err)
	s.Nil(token)
}

func (s *dlqMessageHandlerSuite) TestMergeMessages_ThrowErrorOnDeleteMessages() {
	ackLevel := int64(10)
	lastMessageID := int64(20)
	pageSize := 100
	pageToken := []byte{}
	messageID1 := int64(11)
	messageID2 := int64(12)
	testError := fmt.Errorf("test")
	domainAttribute1 := &types.DomainTaskAttributes{
		ID: uuid.New(),
	}
	domainAttribute2 := &types.DomainTaskAttributes{
		ID: uuid.New(),
	}
	tasks := []*types.ReplicationTask{
		{
			TaskType:             types.ReplicationTaskTypeDomain.Ptr(),
			SourceTaskID:         messageID1,
			DomainTaskAttributes: domainAttribute1,
		},
		{
			TaskType:             types.ReplicationTaskTypeDomain.Ptr(),
			SourceTaskID:         messageID2,
			DomainTaskAttributes: domainAttribute2,
		},
	}
	s.mockReplicationQueue.EXPECT().GetDLQAckLevel(gomock.Any()).Return(ackLevel, nil).Times(1)
	s.mockReplicationQueue.EXPECT().GetMessagesFromDLQ(gomock.Any(), ackLevel, lastMessageID, pageSize, pageToken).
		Return(tasks, nil, nil).Times(1)
	s.mockReplicationTaskExecutor.EXPECT().Execute(domainAttribute1).Return(nil).Times(1)
	s.mockReplicationTaskExecutor.EXPECT().Execute(domainAttribute2).Return(nil).Times(1)
	s.mockReplicationQueue.EXPECT().RangeDeleteMessagesFromDLQ(gomock.Any(), ackLevel, messageID2).Return(testError).Times(1)
	s.mockReplicationQueue.EXPECT().UpdateDLQAckLevel(gomock.Any(), messageID1).Return(nil).Times(1)

	token, err := s.dlqMessageHandler.Merge(context.Background(), lastMessageID, pageSize, pageToken)
	s.Error(err)
	s.Nil(token)
}

func (s *dlqMessageHandlerSuite) TestMergeMessages_IgnoreErrorOnUpdateDLQAckLevel() {
	ackLevel := int64(10)
	lastMessageID := int64(20)
	pageSize := 100
	pageToken := []byte{}
	messageID := int64(11)
	testError := fmt.Errorf("test")
	domainAttribute := &types.DomainTaskAttributes{
		ID: uuid.New(),
	}

	tasks := []*types.ReplicationTask{
		{
			TaskType:             types.ReplicationTaskTypeDomain.Ptr(),
			SourceTaskID:         messageID,
			DomainTaskAttributes: domainAttribute,
		},
	}
	s.mockReplicationQueue.EXPECT().GetDLQAckLevel(gomock.Any()).Return(ackLevel, nil).Times(1)
	s.mockReplicationQueue.EXPECT().GetMessagesFromDLQ(gomock.Any(), ackLevel, lastMessageID, pageSize, pageToken).
		Return(tasks, nil, nil).Times(1)
	s.mockReplicationTaskExecutor.EXPECT().Execute(domainAttribute).Return(nil).Times(1)
	s.mockReplicationQueue.EXPECT().RangeDeleteMessagesFromDLQ(gomock.Any(), ackLevel, messageID).Return(nil).Times(1)
	s.mockReplicationQueue.EXPECT().UpdateDLQAckLevel(gomock.Any(), messageID).Return(testError).Times(1)

	token, err := s.dlqMessageHandler.Merge(context.Background(), lastMessageID, pageSize, pageToken)
	s.NoError(err)
	s.Nil(token)
}
