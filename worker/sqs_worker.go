// Package worker is responsible for communicating with AWS SQS and handing over
// the events to executor for runbook execution if the message passes all the checks.
package worker

import (
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/neptuneio/agent/api"
	"github.com/neptuneio/agent/logging"
	"github.com/neptuneio/agent/security"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

// SQS queue related constants. Using reasonable default for now but we can make them configurable
// if need be in future.
const (
	sqsPollingFrequencySecs            = 5
	maxNumMessagesToFetch              = 10
	longPollTimeSeconds                = 20
	defaultVisibilityTimeout           = 120
	numSQSFailuresBeforeReregistration = 10
)

var queueURLRegex = regexp.MustCompile(`https://sqs\.(.*)\.amazonaws.com(.*)`)
var requiredAttributes []*string

func init() {
	// Agent id and signature are mandatory attributes in every SQS message that agent processes.
	agentIdAttr := "agentId"
	signatureAttr := "signature"
	requiredAttributes = append(requiredAttributes, &agentIdAttr)
	requiredAttributes = append(requiredAttributes, &signatureAttr)
}

// Function to change SQS message visibility.
func changeMessageVisibility(svc *sqs.SQS, queue, receiptHandle string, timeout int64) error {
	params := &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          &queue,
		ReceiptHandle:     &receiptHandle,
		VisibilityTimeout: &timeout,
	}

	_, err := svc.ChangeMessageVisibility(params)

	if err != nil {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		logging.Error("Could not change the message visibility.", logging.Fields{
			"receipt": receiptHandle,
			"error":   err,
		})
		return err
	}

	return nil
}

// Function to delete SQS message after the processing is done.
func DeleteMessage(regInfo *api.RegistrationInfo, receiptHandle *string) error {
	svc := getSQSClient(regInfo)

	logging.Debug("Deleting the event from SQS.", nil)
	params := &sqs.DeleteMessageInput{
		QueueUrl:      &regInfo.ActionQueueEndpoint,
		ReceiptHandle: receiptHandle,
	}
	_, err := svc.DeleteMessage(params)

	if err != nil {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		logging.Error("Could not delete the event.", logging.Fields{"error": err})
		return err
	}

	return nil
}

func parseQueueDetails(queueUrl string) (queue, region string) {
	result := queueURLRegex.FindStringSubmatch(queueUrl)
	return queueUrl, result[1]
}

// Function to poll SQS messages.
func getMessages(svc *sqs.SQS, queue string) (*sqs.ReceiveMessageOutput, error) {
	params := &sqs.ReceiveMessageInput{
		QueueUrl:              &queue,
		MaxNumberOfMessages:   aws.Int64(maxNumMessagesToFetch),
		VisibilityTimeout:     aws.Int64(defaultVisibilityTimeout),
		WaitTimeSeconds:       aws.Int64(longPollTimeSeconds),
		MessageAttributeNames: requiredAttributes,
	}
	logging.Debug("Polling SQS queue for messages.", nil)
	resp, err := svc.ReceiveMessage(params)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func getSQSClient(regInfo *api.RegistrationInfo) *sqs.SQS {
	creds := credentials.NewStaticCredentials(regInfo.AWSAccessKey, regInfo.AWSSecretAccessKey, regInfo.AWSSecurityToken)
	_, region := parseQueueDetails(regInfo.ActionQueueEndpoint)
	awsConfig := aws.NewConfig().WithCredentials(creds).
		WithRegion(region).
		WithHTTPClient(http.DefaultClient).
		WithMaxRetries(aws.UseServiceDefaultRetries).
		WithLogger(aws.NewDefaultLogger()).
		WithLogLevel(aws.LogOff).
		WithSleepDelay(time.Sleep)
	return sqs.New(session.New(awsConfig))
}

// Main worker function which does the following things in an infinite loop.
// 1. Poll for SQS messages using long polling technique.
// 2. Check if the messages received are for this agent, by checking agent id.
//    Release the messages not meant for this agent.
// 3. Verify the signature of the message and delete the message immediately if signature isn't correct.
// 4. Deserialize the event from SQS message.
// 5. Re-verify the agent id (which is inside the payload) again just to double check that agent id attribute
//    was not tampered. This guards against replaying old messages, etc.
// 6. At this point, agent has decided to process the event. So, hide the SQS message for the action timeout
//    and hand over the event to executor for runbook execution.
func RunLoop(regInfo *api.RegistrationInfo, regInfoUpdatesCh <-chan string, eventsChannel chan<- *api.Event, regChannel chan<- time.Time) {

	logging.Info("Initializing SQS client.", nil)
	svc := getSQSClient(regInfo)
	queue := regInfo.ActionQueueEndpoint

	shouldLogError := true
	numFailures := 0
	for {
		shouldSleep := true
		select {

		// Check if the registration info has changed and reinitialize the SQS client if required.
		case <-regInfoUpdatesCh:
			logging.Info("Initializing SQS client.", nil)
			svc = getSQSClient(regInfo)
			queue = regInfo.ActionQueueEndpoint

		default:
			t1 := time.Now()
			if resp, err := getMessages(svc, queue); err == nil {
				shouldLogError = true
				numFailures = 0
				api.UpdateStatus(api.QueuePollingSucceeded)
				logging.Debug("Received messages.", logging.Fields{"count": len(resp.Messages)})

				for _, msg := range resp.Messages {
					bodyStr := *msg.Body
					messageId := *msg.MessageId

					agentId, ok := msg.MessageAttributes["agentId"]
					if !ok {
						logging.Error("Received message does not have agentId attribute.", logging.Fields{"msgId": messageId})
						continue
					}

					if regInfo.AgentId == *agentId.StringValue {
						logging.Debug("Received a message for me. Checking message integrity.", nil)

						signature, ok := msg.MessageAttributes["signature"]

						if !ok {
							logging.Error("Received message does not have signature attribute.", logging.Fields{"msgId": messageId})
							continue
						}

						if valid, err := security.VerifyMessage(bodyStr, *signature.StringValue); valid && err == nil {
							var event api.Event
							err = json.Unmarshal([]byte(bodyStr), &event)
							if err != nil {
								logging.Error("Could not deserialize the SQS message.", logging.Fields{"error": err})
							} else {
								event.SQSMessageId = messageId
								event.ReceiptHandle = *msg.ReceiptHandle
							}

							// Now that the message signature is verified, recheck the agent id from the message payload.
							// This should guard against the cases where someone would have changed the message attributes
							// and set a different agent id in the attributes but didn't tamper with the message.
							if regInfo.AgentId == event.AgentId {

								// Keep a buffer of 2 seconds in addition to the timeout received in the event.
								// This helps to avoid race conditions while handling the action timeout.
								changeMessageVisibility(svc, queue, event.ReceiptHandle, int64(event.Timeout+2))

								// Push into a separate queue so that the action thread picks the message.
								logging.Debug("Pushing the message for processing", logging.Fields{"eventId": event.EventId})
								eventsChannel <- &event
								shouldSleep = false
							} else {
								// This means something is wrong. Ideally the agent id in message attribute and
								// message payload should always match but otherwise, it's an issue.
								// Don't process this message and delete it immediately.
								logging.Error("Something is wrong!! Agent id present in the message attributes matches but "+
									"agent id in event does not match. Deleting the message.",
									logging.Fields{"msgId": messageId})
								DeleteMessage(regInfo, msg.ReceiptHandle)
							}
						} else {
							logging.Error("Could not verify the message with signature so deleting the message.",
								logging.Fields{"msgId": messageId, "error": err})
							DeleteMessage(regInfo, msg.ReceiptHandle)
						}
					} else {
						logging.Debug("Releasing a message which is not for me.", logging.Fields{"msgId": messageId})
						changeMessageVisibility(svc, queue, *msg.ReceiptHandle, int64(0))
					}
				}
			} else if shouldLogError {
				// Print the error, cast err to awserr.Error to get the Code and
				// Message from an error.
				logging.Error("Could not receive messages from SQS.", logging.Fields{
					"error":    err,
					"response": resp,
				})
				shouldLogError = false
				numFailures += 1
			} else {
				numFailures += 1

				// Re-trigger the registration if we fail to poll the queue 10 times in succession.
				if numFailures == numSQSFailuresBeforeReregistration {
					numFailures = 0
					shouldLogError = true
					regChannel <- time.Now()
				}
			}

			// Sleep if required. We make sure there is at least sqsPollingFrequencySecs gap between successive SQS polls.
			if shouldSleep {
				if duration := t1.Add(time.Second * sqsPollingFrequencySecs).Sub(time.Now()); duration > 0 {
					logging.Debug("Sleeping between two polls.", logging.Fields{"duration": duration})
					time.Sleep(duration)
				}
			}
		}
	}
}
