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

const UPSTREAM_CHECKIN_SUPPORTED_PRESET_IDS = new Set([
  'new-api',
  'one-api',
  'one-hub',
  'veloera',
  'anyrouter',
]);

const UPSTREAM_CHECKIN_SUPPORTED_PLATFORMS = new Set([
  'new-api',
  'one-api',
  'one-hub',
  'veloera',
  'anyrouter',
]);

export const parseChannelJsonObject = (rawValue) => {
  if (rawValue && typeof rawValue === 'object') {
    return rawValue;
  }

  if (typeof rawValue !== 'string' || !rawValue.trim()) {
    return {};
  }

  try {
    const parsed = JSON.parse(rawValue);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch (error) {
    return {};
  }
};

export const getUpstreamCheckinInfo = (rawOtherInfo) => {
  const parsedOtherInfo = parseChannelJsonObject(rawOtherInfo);
  return {
    status: parsedOtherInfo.upstream_checkin_status || 'unchecked',
    checkedAt: Number(parsedOtherInfo.upstream_checkin_time) || 0,
    trigger: parsedOtherInfo.upstream_checkin_trigger || '',
    reason: parsedOtherInfo.upstream_checkin_reason || '',
    reward: parsedOtherInfo.upstream_checkin_reward || '',
    message: parsedOtherInfo.upstream_checkin_message || '',
  };
};

export const getUpstreamCheckinStatusMeta = (status, t) => {
  switch (status) {
    case 'success':
      return { color: 'green', label: t('签到成功') };
    case 'failed':
      return { color: 'red', label: t('签到失败') };
    case 'skipped':
      return { color: 'grey', label: t('已跳过') };
    default:
      return { color: 'light-blue', label: t('未执行') };
  }
};

export const getChannelUpstreamCheckinState = (record) => {
  const settings = parseChannelJsonObject(record?.settings);
  const info = getUpstreamCheckinInfo(record?.other_info);
  const presetId = String(settings.upstream_preset || '').trim();
  const platform = String(settings.upstream_platform || '').trim();
  const hasToken = String(settings.upstream_checkin_access_token || '').trim() !== '';
  const hasUserID = Number(settings.upstream_checkin_user_id) > 0;
  const autoEnabled = settings.upstream_checkin_enabled === true;
  const hasHistory =
    info.checkedAt > 0 ||
    info.status !== 'unchecked' ||
    Boolean(info.reason || info.reward || info.message || info.trigger);
  const supportedPreset =
    UPSTREAM_CHECKIN_SUPPORTED_PRESET_IDS.has(presetId) ||
    UPSTREAM_CHECKIN_SUPPORTED_PLATFORMS.has(platform);
  const configured = hasToken;

  return {
    settings,
    info,
    supportedPreset,
    configured,
    available: supportedPreset || hasToken || hasUserID || autoEnabled || hasHistory,
  };
};
