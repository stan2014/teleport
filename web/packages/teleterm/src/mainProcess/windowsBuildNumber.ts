import { RuntimeSettings } from 'teleterm/mainProcess/types';

export function getWindowsBuildNumber(
  runtimeSettings: RuntimeSettings
): number {
  if (runtimeSettings.platform !== 'win32') {
    return;
  }

  const osVersion = /(\d+)\.(\d+)\.(\d+)/g.exec(runtimeSettings.osVersion);
  let buildNumber: number;
  if (osVersion && osVersion.length === 4) {
    buildNumber = parseInt(osVersion[3]);
  }
  return buildNumber;
}
