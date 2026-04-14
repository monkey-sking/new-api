/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useMemo, useState } from 'react';
import {
  API,
  showError,
  showInfo,
  showSuccess,
  timestamp2string,
} from '../../../../helpers';
import { useTranslation } from 'react-i18next';
import {
  Avatar,
  Banner,
  Button,
  Card,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Switch,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconBolt,
  IconDelete,
  IconGlobe,
  IconPlus,
  IconRefresh,
  IconSearch,
} from '@douyinfe/semi-icons';
import CardPro from '../../../common/ui/CardPro';
import CardTable from '../../../common/ui/CardTable';

const { Text } = Typography;

const defaultSiteForm = {
  id: 0,
  name: '',
  base_url: '',
  platform: '',
  preset: '',
  proxy: '',
  status: 'active',
};

const defaultAccountForm = {
  id: 0,
  site_id: 0,
  name: '',
  username: '',
  password: '',
  access_token: '',
  checkin_enabled: true,
  checkin_interval_hours: 24,
  status: 'active',
};

const platformOptions = [
  'new-api',
  'one-api',
  'one-hub',
  'veloera',
  'anyrouter',
  'done-hub',
  'sub2api',
  'cliproxyapi',
  'openai',
  'claude',
  'gemini',
  'openrouter',
];

const statusColorMap = {
  active: 'green',
  success: 'green',
  skipped: 'blue',
  expired: 'orange',
  failed: 'red',
  disabled: 'grey',
};

const UpstreamOperationsPage = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [sites, setSites] = useState([]);
  const [accounts, setAccounts] = useState([]);
  const [logs, setLogs] = useState([]);
  const [siteModalVisible, setSiteModalVisible] = useState(false);
  const [accountModalVisible, setAccountModalVisible] = useState(false);
  const [siteSubmitting, setSiteSubmitting] = useState(false);
  const [accountSubmitting, setAccountSubmitting] = useState(false);
  const [siteDetecting, setSiteDetecting] = useState(false);
  const [refreshingSessionId, setRefreshingSessionId] = useState(0);
  const [rowCheckinLoadingMap, setRowCheckinLoadingMap] = useState({});
  const [siteForm, setSiteForm] = useState(defaultSiteForm);
  const [accountForm, setAccountForm] = useState(defaultAccountForm);

  const siteOptions = useMemo(
    () =>
      sites.map((site) => ({
        label: site.name || site.base_url || `#${site.id}`,
        value: site.id,
      })),
    [sites],
  );

  const siteNameMap = useMemo(() => {
    const nextMap = {};
    sites.forEach((site) => {
      nextMap[site.id] = site;
    });
    return nextMap;
  }, [sites]);

  const fetchData = async ({ silent = false } = {}) => {
    if (silent) {
      setRefreshing(true);
    } else {
      setLoading(true);
    }

    try {
      const [sitesRes, accountsRes, logsRes] = await Promise.all([
        API.get('/api/upstream/sites?p=1&page_size=100', { skipErrorHandler: true }),
        API.get('/api/upstream/accounts?p=1&page_size=100', { skipErrorHandler: true }),
        API.get('/api/upstream/checkin_logs?p=1&page_size=100', { skipErrorHandler: true }),
      ]);

      const ensureSuccess = (response, fallbackMessage) => {
        if (!response?.data?.success) {
          throw new Error(response?.data?.message || fallbackMessage);
        }
        return response.data.data || {};
      };

      const sitesPayload = ensureSuccess(sitesRes, t('加载上游站点失败'));
      const accountsPayload = ensureSuccess(accountsRes, t('加载上游账号失败'));
      const logsPayload = ensureSuccess(logsRes, t('加载签到记录失败'));

      setSites(Array.isArray(sitesPayload.items) ? sitesPayload.items : []);
      setAccounts(Array.isArray(accountsPayload.items) ? accountsPayload.items : []);
      setLogs(Array.isArray(logsPayload.items) ? logsPayload.items : []);
    } catch (error) {
      showError(error.message || t('加载上游运营数据失败'));
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    fetchData().then();
  }, []);

  const normalizedKeyword = keyword.trim().toLowerCase();

  const filteredSites = useMemo(() => {
    if (!normalizedKeyword) {
      return sites;
    }
    return sites.filter((site) =>
      [site.name, site.base_url, site.platform, site.preset]
        .map((value) => String(value || '').toLowerCase())
        .some((value) => value.includes(normalizedKeyword)),
    );
  }, [normalizedKeyword, sites]);

  const filteredAccounts = useMemo(() => {
    if (!normalizedKeyword) {
      return accounts;
    }
    return accounts.filter((account) => {
      const site = siteNameMap[account.site_id] || account.site;
      return [
        account.name,
        account.username,
        account.status,
        account.last_checkin_status,
        account.last_checkin_message,
        site?.name,
        site?.base_url,
      ]
        .map((value) => String(value || '').toLowerCase())
        .some((value) => value.includes(normalizedKeyword));
    });
  }, [accounts, normalizedKeyword, siteNameMap]);

  const filteredLogs = useMemo(() => {
    if (!normalizedKeyword) {
      return logs;
    }
    return logs.filter((log) => {
      const site = log.account?.site;
      return [
        log.status,
        log.message,
        log.reward,
        log.account?.name,
        log.account?.username,
        site?.name,
        site?.base_url,
      ]
        .map((value) => String(value || '').toLowerCase())
        .some((value) => value.includes(normalizedKeyword));
    });
  }, [logs, normalizedKeyword]);

  const stats = useMemo(() => {
    const autoEnabledAccounts = accounts.filter(
      (account) => account.checkin_enabled === true,
    ).length;
    const failedLogs = logs.filter((log) => log.status === 'failed').length;
    return {
      sites: sites.length,
      accounts: accounts.length,
      autoEnabledAccounts,
      failedLogs,
    };
  }, [accounts, logs, sites.length]);

  const updateSiteForm = (key, value) => {
    setSiteForm((prev) => ({
      ...prev,
      [key]: value,
    }));
  };

  const updateAccountForm = (key, value) => {
    setAccountForm((prev) => ({
      ...prev,
      [key]: value,
    }));
  };

  const openCreateSiteModal = () => {
    setSiteForm(defaultSiteForm);
    setSiteModalVisible(true);
  };

  const openEditSiteModal = (site) => {
    setSiteForm({
      id: site.id,
      name: site.name || '',
      base_url: site.base_url || '',
      platform: site.platform || '',
      preset: site.preset || '',
      proxy: site.proxy || '',
      status: site.status || 'active',
    });
    setSiteModalVisible(true);
  };

  const openCreateAccountModal = () => {
    if (sites.length === 0) {
      showInfo(t('请先添加上游站点'));
      return;
    }
    setAccountForm({
      ...defaultAccountForm,
      site_id: sites[0]?.id || 0,
    });
    setAccountModalVisible(true);
  };

  const openEditAccountModal = (account) => {
    setAccountForm({
      id: account.id,
      site_id: account.site_id,
      name: account.name || '',
      username: account.username || '',
      password: '',
      access_token: '',
      checkin_enabled: account.checkin_enabled === true,
      checkin_interval_hours: account.checkin_interval_hours || 24,
      status: account.status || 'active',
    });
    setAccountModalVisible(true);
  };

  const detectSitePreset = async () => {
    if (!siteForm.base_url || siteForm.base_url.trim() === '') {
      showInfo(t('先填写站点地址'));
      return;
    }
    setSiteDetecting(true);
    try {
      const res = await API.post(
        '/api/channel/upstream/detect',
        {
          base_url: siteForm.base_url,
          type: 0,
        },
        { skipErrorHandler: true },
      );
      if (!res?.data?.success) {
        throw new Error(res?.data?.message || t('自动识别失败'));
      }
      const detection = res.data?.data?.detection || {};
      if (detection.normalized_base_url) {
        updateSiteForm('base_url', detection.normalized_base_url);
      }
      if (detection.platform) {
        updateSiteForm('platform', detection.platform);
      }
      if (detection.preset?.id) {
        updateSiteForm('preset', detection.preset.id);
      }
      showSuccess(t('已更新站点预设'));
    } catch (error) {
      showError(error.message || t('自动识别失败'));
    } finally {
      setSiteDetecting(false);
    }
  };

  const submitSite = async () => {
    setSiteSubmitting(true);
    try {
      const payload = {
        id: siteForm.id,
        name: siteForm.name,
        base_url: siteForm.base_url,
        platform: siteForm.platform,
        preset: siteForm.preset,
        proxy: siteForm.proxy,
        status: siteForm.status,
      };
      const request = siteForm.id > 0
        ? API.put('/api/upstream/sites', payload, { skipErrorHandler: true })
        : API.post('/api/upstream/sites', payload, { skipErrorHandler: true });
      const res = await request;
      if (!res?.data?.success) {
        throw new Error(res?.data?.message || t('保存上游站点失败'));
      }
      showSuccess(siteForm.id > 0 ? t('站点已更新') : t('站点已创建'));
      setSiteModalVisible(false);
      setSiteForm(defaultSiteForm);
      await fetchData({ silent: true });
    } catch (error) {
      showError(error.message || t('保存上游站点失败'));
    } finally {
      setSiteSubmitting(false);
    }
  };

  const submitAccount = async () => {
    setAccountSubmitting(true);
    try {
      const payload = {
        id: accountForm.id,
        site_id: accountForm.site_id,
        name: accountForm.name,
        username: accountForm.username,
        password: accountForm.password,
        access_token: accountForm.access_token,
        checkin_enabled: accountForm.checkin_enabled,
        checkin_interval_hours: accountForm.checkin_interval_hours,
        status: accountForm.status,
      };
      const request = accountForm.id > 0
        ? API.put('/api/upstream/accounts', payload, { skipErrorHandler: true })
        : API.post('/api/upstream/accounts', payload, { skipErrorHandler: true });
      const res = await request;
      if (!res?.data?.success) {
        throw new Error(res?.data?.message || t('保存上游账号失败'));
      }
      showSuccess(accountForm.id > 0 ? t('账号已更新') : t('账号已创建'));
      setAccountModalVisible(false);
      setAccountForm(defaultAccountForm);
      await fetchData({ silent: true });
    } catch (error) {
      showError(error.message || t('保存上游账号失败'));
    } finally {
      setAccountSubmitting(false);
    }
  };

  const deleteSite = (site) => {
    Modal.confirm({
      title: t('删除站点'),
      content: t('删除后，该站点下的上游账号和签到记录也会一起删除。'),
      onOk: async () => {
        const res = await API.delete(`/api/upstream/sites/${site.id}`, {
          skipErrorHandler: true,
        });
        if (!res?.data?.success) {
          showError(res?.data?.message || t('删除站点失败'));
          return;
        }
        showSuccess(t('站点已删除'));
        fetchData({ silent: true }).then();
      },
    });
  };

  const deleteAccount = (account) => {
    Modal.confirm({
      title: t('删除账号'),
      content: t('删除后，这个账号的签到历史也会一起删除。'),
      onOk: async () => {
        const res = await API.delete(`/api/upstream/accounts/${account.id}`, {
          skipErrorHandler: true,
        });
        if (!res?.data?.success) {
          showError(res?.data?.message || t('删除账号失败'));
          return;
        }
        showSuccess(t('账号已删除'));
        fetchData({ silent: true }).then();
      },
    });
  };

  const refreshAccountSession = async (account) => {
    setRefreshingSessionId(account.id);
    try {
      const res = await API.post(
        `/api/upstream/accounts/${account.id}/refresh_session`,
        {},
        { skipErrorHandler: true },
      );
      if (!res?.data?.success) {
        throw new Error(res?.data?.message || t('刷新会话失败'));
      }
      const session = res.data?.data?.session || {};
      const successMessage =
        session.platform_user_id > 0
          ? t('会话已更新，自动识别 user id = {{userId}}', {
              userId: session.platform_user_id,
            })
          : t('会话已更新');
      showSuccess(successMessage);
      await fetchData({ silent: true });
    } catch (error) {
      showError(error.message || t('刷新会话失败'));
    } finally {
      setRefreshingSessionId(0);
    }
  };

  const runAccountCheckin = async (account) => {
    setRowCheckinLoadingMap((prev) => ({
      ...prev,
      [account.id]: true,
    }));
    try {
      const res = await API.post(
        `/api/upstream/accounts/${account.id}/checkin`,
        {},
        { skipErrorHandler: true },
      );
      if (!res?.data?.success) {
        throw new Error(res?.data?.message || t('执行签到失败'));
      }
      showSuccess(t('签到已执行'));
      await fetchData({ silent: true });
    } catch (error) {
      showError(error.message || t('执行签到失败'));
    } finally {
      setRowCheckinLoadingMap((prev) => ({
        ...prev,
        [account.id]: false,
      }));
    }
  };

  const siteColumns = [
    {
      title: t('站点'),
      dataIndex: 'name',
      key: 'name',
      render: (text, record) => (
        <div className='min-w-[240px]'>
          <div className='font-medium'>{text || `#${record.id}`}</div>
          <div className='text-xs text-gray-500 break-all mt-1'>
            {record.base_url || '-'}
          </div>
        </div>
      ),
    },
    {
      title: t('平台'),
      dataIndex: 'platform',
      key: 'platform',
      render: (text, record) => (
        <Space spacing={6} wrap>
          <Tag color='blue'>{text || t('未识别')}</Tag>
          {record.preset ? (
            <Tag color='white' type='ghost'>
              {record.preset}
            </Tag>
          ) : null}
        </Space>
      ),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      key: 'status',
      render: (text) => (
        <Tag color={statusColorMap[text] || 'grey'}>
          {text || t('未知')}
        </Tag>
      ),
    },
    {
      title: '',
      dataIndex: 'operate',
      key: 'operate',
      render: (text, record) => (
        <Space wrap>
          <Button size='small' type='tertiary' onClick={() => openEditSiteModal(record)}>
            {t('编辑')}
          </Button>
          <Button
            size='small'
            type='tertiary'
            icon={<IconDelete />}
            onClick={() => deleteSite(record)}
          >
            {t('删除')}
          </Button>
        </Space>
      ),
    },
  ];

  const accountColumns = [
    {
      title: t('账号'),
      dataIndex: 'name',
      key: 'name',
      render: (text, record) => (
        <div className='min-w-[220px]'>
          <div className='font-medium'>{text || `#${record.id}`}</div>
          <div className='text-xs text-gray-500 mt-1'>
            {record.username || t('未填写用户名')}
          </div>
          <div className='text-xs text-gray-500 mt-1'>
            {(record.site && record.site.name) ||
              siteNameMap[record.site_id]?.name ||
              '-'}
          </div>
        </div>
      ),
    },
    {
      title: t('会话'),
      dataIndex: 'session',
      key: 'session',
      render: (text, record) => (
        <div className='min-w-[220px]'>
          <Space spacing={6} wrap>
            <Tag color={statusColorMap[record.status] || 'grey'}>
              {record.status || t('未知')}
            </Tag>
            {record.checkin_enabled === true ? (
              <Tag color='green'>{t('自动签到')}</Tag>
            ) : (
              <Tag color='grey'>{t('仅手动')}</Tag>
            )}
          </Space>
          <div className='text-xs text-gray-500 mt-2'>
            {t('user id')}: {record.platform_user_id || '-'}
          </div>
          <div className='text-xs text-gray-500 mt-1 break-all'>
            API Token: {record.api_token || '-'}
          </div>
        </div>
      ),
    },
    {
      title: t('最近签到'),
      dataIndex: 'last_checkin_status',
      key: 'last_checkin',
      render: (text, record) => (
        <div className='min-w-[240px]'>
          <Space spacing={6} wrap>
            <Tag color={statusColorMap[text] || 'grey'}>
              {text || t('未执行')}
            </Tag>
            {record.last_checkin_reward ? (
              <Tag color='blue'>{record.last_checkin_reward}</Tag>
            ) : null}
          </Space>
          <div className='text-xs text-gray-500 mt-2'>
            {record.last_checkin_at > 0
              ? timestamp2string(record.last_checkin_at)
              : '-'}
          </div>
          <Tooltip content={record.last_checkin_message || '-'}>
            <div className='text-xs text-gray-500 mt-1 truncate max-w-[240px]'>
              {record.last_checkin_message || '-'}
            </div>
          </Tooltip>
        </div>
      ),
    },
    {
      title: '',
      dataIndex: 'operate',
      key: 'operate',
      render: (text, record) => (
        <Space wrap>
          <Button
            size='small'
            type='primary'
            loading={refreshingSessionId === record.id}
            onClick={() => refreshAccountSession(record)}
          >
            {t('刷新会话')}
          </Button>
          <Button
            size='small'
            type='tertiary'
            loading={rowCheckinLoadingMap[record.id] === true}
            onClick={() => runAccountCheckin(record)}
          >
            {t('签到')}
          </Button>
          <Button size='small' type='tertiary' onClick={() => openEditAccountModal(record)}>
            {t('编辑')}
          </Button>
          <Button
            size='small'
            type='tertiary'
            icon={<IconDelete />}
            onClick={() => deleteAccount(record)}
          >
            {t('删除')}
          </Button>
        </Space>
      ),
    },
  ];

  const logColumns = [
    {
      title: t('时间'),
      dataIndex: 'created_time',
      key: 'created_time',
      render: (text) => (text > 0 ? timestamp2string(text) : '-'),
    },
    {
      title: t('账号'),
      dataIndex: 'account',
      key: 'account',
      render: (text, record) => (
        <div className='min-w-[220px]'>
          <div className='font-medium'>{record.account?.name || '-'}</div>
          <div className='text-xs text-gray-500 mt-1'>
            {record.account?.site?.name || '-'}
          </div>
        </div>
      ),
    },
    {
      title: t('结果'),
      dataIndex: 'status',
      key: 'status',
      render: (text, record) => (
        <Space spacing={6} wrap>
          <Tag color={statusColorMap[text] || 'grey'}>
            {text || t('未知')}
          </Tag>
          {record.reward ? <Tag color='blue'>{record.reward}</Tag> : null}
          {record.trigger ? (
            <Tag color='white' type='ghost'>
              {record.trigger}
            </Tag>
          ) : null}
        </Space>
      ),
    },
    {
      title: t('说明'),
      dataIndex: 'message',
      key: 'message',
      render: (text) => (
        <Tooltip content={text || '-'}>
          <div className='truncate max-w-[320px]'>{text || '-'}</div>
        </Tooltip>
      ),
    },
  ];

  const statCards = [
    {
      title: t('上游站点'),
      value: stats.sites,
      description: t('独立维护上游面板地址和平台类型'),
      className: 'bg-[linear-gradient(135deg,#f8fffb_0%,#eefcf7_100%)]',
    },
    {
      title: t('上游账号'),
      value: stats.accounts,
      description: t('账号登录后自动获取会话和 user id'),
      className: 'bg-[linear-gradient(135deg,#fffaf2_0%,#fff2de_100%)]',
    },
    {
      title: t('自动签到中'),
      value: stats.autoEnabledAccounts,
      description: t('定时任务会按账号间隔自动执行签到'),
      className: 'bg-[linear-gradient(135deg,#f4f8ff_0%,#ebf2ff_100%)]',
    },
    {
      title: t('最近失败'),
      value: stats.failedLogs,
      description: t('优先排查会话过期、站点限制或接口变更'),
      className: 'bg-[linear-gradient(135deg,#fff6f6_0%,#ffe8e8_100%)]',
    },
  ];

  return (
    <>
      <Modal
        title={siteForm.id > 0 ? t('编辑上游站点') : t('添加上游站点')}
        visible={siteModalVisible}
        onCancel={() => {
          setSiteModalVisible(false);
          setSiteForm(defaultSiteForm);
        }}
        onOk={submitSite}
        confirmLoading={siteSubmitting}
        okText={t('保存')}
        cancelText={t('取消')}
      >
        <div className='space-y-3'>
          <Input
            value={siteForm.name}
            onChange={(value) => updateSiteForm('name', value)}
            placeholder={t('站点名称')}
          />
          <Input
            value={siteForm.base_url}
            onChange={(value) => updateSiteForm('base_url', value)}
            addonAfter={
              <Button
                theme='borderless'
                loading={siteDetecting}
                onClick={detectSitePreset}
              >
                {t('自动识别')}
              </Button>
            }
            placeholder={t('例如：https://panel.example.com')}
          />
          <Select
            value={siteForm.platform}
            onChange={(value) => updateSiteForm('platform', value)}
            placeholder={t('平台类型')}
          >
            {platformOptions.map((platform) => (
              <Select.Option key={platform} value={platform}>
                {platform}
              </Select.Option>
            ))}
          </Select>
          <Input
            value={siteForm.preset}
            onChange={(value) => updateSiteForm('preset', value)}
            placeholder={t('预设 ID，可自动识别后回填')}
          />
          <Input
            value={siteForm.proxy}
            onChange={(value) => updateSiteForm('proxy', value)}
            placeholder={t('代理地址，可选')}
          />
          <Select
            value={siteForm.status}
            onChange={(value) => updateSiteForm('status', value)}
          >
            <Select.Option value='active'>{t('启用')}</Select.Option>
            <Select.Option value='disabled'>{t('停用')}</Select.Option>
          </Select>
        </div>
      </Modal>

      <Modal
        title={accountForm.id > 0 ? t('编辑上游账号') : t('添加上游账号')}
        visible={accountModalVisible}
        onCancel={() => {
          setAccountModalVisible(false);
          setAccountForm(defaultAccountForm);
        }}
        onOk={submitAccount}
        confirmLoading={accountSubmitting}
        okText={t('保存')}
        cancelText={t('取消')}
      >
        <div className='space-y-3'>
          <Banner
            type='info'
            description={t(
              '支持两种方式：填用户名和密码后点“刷新会话”自动拿会话，或直接手动导入 Access Token。',
            )}
          />
          <Select
            value={accountForm.site_id}
            onChange={(value) => updateAccountForm('site_id', value)}
            placeholder={t('选择所属站点')}
          >
            {siteOptions.map((option) => (
              <Select.Option key={option.value} value={option.value}>
                {option.label}
              </Select.Option>
            ))}
          </Select>
          <Input
            value={accountForm.name}
            onChange={(value) => updateAccountForm('name', value)}
            placeholder={t('账号名称')}
          />
          <Input
            value={accountForm.username}
            onChange={(value) => updateAccountForm('username', value)}
            placeholder={t('用户名')}
          />
          <Input
            value={accountForm.password}
            onChange={(value) => updateAccountForm('password', value)}
            mode='password'
            autoComplete='new-password'
            placeholder={
              accountForm.id > 0
                ? t('留空表示保留原密码')
                : t('密码，可用于自动获取会话')
            }
          />
          <Input
            value={accountForm.access_token}
            onChange={(value) => updateAccountForm('access_token', value)}
            mode='password'
            autoComplete='new-password'
            placeholder={
              accountForm.id > 0
                ? t('留空表示保留原 Access Token')
                : t('手动导入 Access Token')
            }
          />
          <div className='flex items-center justify-between rounded-xl bg-gray-50 px-3 py-3'>
            <div>
              <div className='font-medium text-sm'>{t('启用自动签到')}</div>
              <div className='text-xs text-gray-500 mt-1'>
                {t('按账号间隔自动执行，不再挂在渠道表单里')}
              </div>
            </div>
            <Switch
              checked={accountForm.checkin_enabled}
              onChange={(checked) => updateAccountForm('checkin_enabled', checked)}
            />
          </div>
          <InputNumber
            value={accountForm.checkin_interval_hours}
            min={1}
            style={{ width: '100%' }}
            onNumberChange={(value) =>
              updateAccountForm('checkin_interval_hours', value || 24)
            }
            placeholder={t('签到间隔（小时）')}
          />
          <Select
            value={accountForm.status}
            onChange={(value) => updateAccountForm('status', value)}
          >
            <Select.Option value='active'>{t('启用')}</Select.Option>
            <Select.Option value='disabled'>{t('停用')}</Select.Option>
            <Select.Option value='expired'>{t('会话过期')}</Select.Option>
          </Select>
        </div>
      </Modal>

      <CardPro
        type='type1'
        t={t}
        descriptionArea={
          <div className='space-y-4'>
            <div className='flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between'>
              <div>
                <div className='flex items-center gap-2'>
                  <Avatar color='blue' size='small'>
                    <IconGlobe size={16} />
                  </Avatar>
                  <Text className='text-lg font-medium'>{t('上游运营')}</Text>
                </div>
                <div className='text-sm text-gray-500 mt-2'>
                  {t(
                    '现在这里按“站点 -> 账号 -> 签到记录”管理，不再把签到藏在渠道字段里。',
                  )}
                </div>
              </div>
              <Space wrap>
                <Button
                  icon={<IconRefresh size={14} />}
                  loading={refreshing}
                  onClick={() => fetchData({ silent: true })}
                >
                  {t('刷新')}
                </Button>
                <Button
                  type='primary'
                  theme='solid'
                  icon={<IconPlus size={14} />}
                  onClick={openCreateSiteModal}
                >
                  {t('添加站点')}
                </Button>
                <Button
                  type='tertiary'
                  icon={<IconBolt size={14} />}
                  onClick={openCreateAccountModal}
                >
                  {t('添加账号')}
                </Button>
              </Space>
            </div>

            <Banner
              type='info'
              className='!rounded-2xl'
              description={t(
                '推荐流程：先添加站点并自动识别平台，再添加账号，最后点“刷新会话”自动获取 access token、API token 和 user id。',
              )}
            />

            <div className='grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4'>
              {statCards.map((card) => (
                <Card
                  key={card.title}
                  className={`!rounded-2xl shadow-sm border-0 ${card.className}`}
                >
                  <div className='text-sm text-gray-500'>{card.title}</div>
                  <div className='text-3xl font-semibold mt-2'>{card.value}</div>
                  <div className='text-xs text-gray-500 mt-2'>{card.description}</div>
                </Card>
              ))}
            </div>
          </div>
        }
        actionsArea={
          <div className='flex flex-col gap-2 md:flex-row md:items-center md:justify-between'>
            <Input
              prefix={<IconSearch />}
              placeholder={t('搜索站点、账号、日志说明')}
              value={keyword}
              onChange={setKeyword}
              showClear
            />
          </div>
        }
      >
        <div className='space-y-4'>
          <Card className='!rounded-2xl shadow-sm border-0'>
            <div className='flex items-center justify-between gap-3 mb-3'>
              <div>
                <div className='text-base font-medium'>{t('上游站点')}</div>
                <div className='text-sm text-gray-500 mt-1'>
                  {t('把平台类型、站点地址和代理放到独立站点层管理。')}
                </div>
              </div>
              <Tag color='blue'>{filteredSites.length}</Tag>
            </div>
            <CardTable
              rowKey='id'
              columns={siteColumns}
              dataSource={filteredSites}
              loading={loading}
              hidePagination
            />
          </Card>

          <Card className='!rounded-2xl shadow-sm border-0'>
            <div className='flex items-center justify-between gap-3 mb-3'>
              <div>
                <div className='text-base font-medium'>{t('上游账号')}</div>
                <div className='text-sm text-gray-500 mt-1'>
                  {t('刷新会话后会自动补 access token、API token 和 user id。')}
                </div>
              </div>
              <Tag color='green'>{filteredAccounts.length}</Tag>
            </div>
            <CardTable
              rowKey='id'
              columns={accountColumns}
              dataSource={filteredAccounts}
              loading={loading}
              hidePagination
            />
          </Card>

          <Card className='!rounded-2xl shadow-sm border-0'>
            <div className='flex items-center justify-between gap-3 mb-3'>
              <div>
                <div className='text-base font-medium'>{t('签到记录')}</div>
                <div className='text-sm text-gray-500 mt-1'>
                  {t('每次签到都会落历史，不再只看渠道上的最后一次结果。')}
                </div>
              </div>
              <Tag color='orange'>{filteredLogs.length}</Tag>
            </div>
            <CardTable
              rowKey='id'
              columns={logColumns}
              dataSource={filteredLogs}
              loading={loading}
              hidePagination
            />
          </Card>
        </div>
      </CardPro>
    </>
  );
};

export default UpstreamOperationsPage;
