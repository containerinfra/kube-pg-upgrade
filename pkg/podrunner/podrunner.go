package podrunner

import (
	"bufio"
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

type Client struct {
	k8sClient clientset.Interface
}

func NewPodRunner(clusterK8sClient clientset.Interface) *Client {
	return &Client{
		k8sClient: clusterK8sClient,
	}
}

func (c *Client) RunPod(ctx context.Context, namespace, name string, taskPod v1.Pod) error {
	podsApi := c.k8sClient.CoreV1().Pods(namespace)

	if err := c.cleanUpTask(ctx, namespace, name); err != nil && !kubeerrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete existing task pod %q: %w", name, err)
	}

	defer c.cleanUpTask(ctx, namespace, name)

	_, err := podsApi.Create(ctx, &taskPod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create job %q: %w", name, err)
	}

	err = c.waitForPodToStart(ctx, namespace, name)
	if err != nil {
		return err
	}
	err = c.tailPodLogs(ctx, namespace, name)
	if err != nil {
		return err
	}
	err = c.waitForPodToComplete(ctx, namespace, name)
	if err != nil {
		return err
	}
	fmt.Printf("task pod %q has completed\n", name)

	// Clean-up once the job has been completed
	if err := c.cleanUpTask(ctx, namespace, name); err != nil && !kubeerrors.IsNotFound(err) {
		return fmt.Errorf("failed to clean-up upgrade pod %q: %w", name, err)
	}
	return nil
}

func (c *Client) cleanUpTask(ctx context.Context, namespace, jobName string) error {
	upgradeNodeJob := c.k8sClient.CoreV1().Pods(namespace)

	background := metav1.DeletePropagationBackground
	err := upgradeNodeJob.Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: &background})
	if err != nil && !kubeerrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete existing upgrade task %q: %w", jobName, err)
	}
	return nil
}

func (c *Client) tailPodLogs(ctx context.Context, namespace, jobName string) error {
	req := c.k8sClient.CoreV1().Pods(namespace).GetLogs(jobName, &v1.PodLogOptions{
		Follow: true,
	})

	podLogs, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer podLogs.Close()

	reader := bufio.NewScanner(podLogs)
	var logLine string
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if reader.Scan() {
				logLine = reader.Text()
				fmt.Printf("[%v]: %v\n", jobName, logLine)
			} else {
				return reader.Err()
			}
		}
	}
}

func (c *Client) waitForPodToStart(ctx context.Context, namespace, jobName string) error {
	for {
		pod, err := c.k8sClient.CoreV1().Pods(namespace).Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Running != nil || containerStatus.State.Terminated != nil {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			continue
		}
	}
}

func (c *Client) waitForPodToComplete(ctx context.Context, namespace, jobName string) error {
	for {
		pod, err := c.k8sClient.CoreV1().Pods(namespace).Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Terminated != nil {
				if containerStatus.State.Terminated.ExitCode == 0 {
					return nil
				} else {
					return fmt.Errorf("unexpected exit code: %d", containerStatus.State.Terminated.ExitCode)
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
			continue
		}
	}
}
