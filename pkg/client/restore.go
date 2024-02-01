package client

import (
	console "github.com/pluralsh/console-client-go"
)

func (c *Client) GetClusterRestore(id string) (*console.ClusterRestoreFragment, error) {
	restore, err := c.consoleClient.GetClusterRestore(c.ctx, id)
	if err != nil {
		return nil, err
	}

	return restore.ClusterRestore, nil
}

func (c *Client) UpdateClusterRestore(id string, attrs console.RestoreAttributes) (*console.ClusterRestoreFragment, error) {
	restore, err := c.consoleClient.UpdateClusterRestore(c.ctx, id, attrs)
	if err != nil {
		return nil, err
	}

	return restore.UpdateClusterRestore, nil
}

func (c *Client) CreateClusterBackup(attrs console.BackupAttributes) (*console.ClusterBackupFragment, error) {
	backup, err := c.consoleClient.CreateClusterBackup(c.ctx, attrs)
	if err != nil {
		return nil, err
	}

	return backup.CreateClusterBackup, nil
}

func (c *Client) GetClusterBackup(clusterID, namespace, name string) (*console.ClusterBackupFragment, error) {
	backup, err := c.consoleClient.GetClusterBackup(c.ctx, nil, &clusterID, &namespace, &name)
	if err != nil {
		return nil, err
	}

	return backup.ClusterBackup, nil
}
