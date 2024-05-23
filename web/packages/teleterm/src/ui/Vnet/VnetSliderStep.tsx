/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { StepComponentProps } from 'design/StepSlider';
import { Box, Flex, Text } from 'design';
import { mergeRefs } from 'shared/libs/mergeRefs';
import { useRefAutoFocus } from 'shared/hooks';

import { useVnetContext } from './vnetContext';
import { VnetSliderStepHeader, AppConnectionItem } from './VnetConnectionItem';

/**
 * VnetSliderStep is the second step of StepSlider used in TopBar/Connections. It is shown after
 * selecting VnetConnectionItem from ConnectionsFilterableList.
 */
export const VnetSliderStep = (props: StepComponentProps) => {
  const visible = props.stepIndex === 1 && props.hasTransitionEnded;
  const { status, startAttempt, stopAttempt } = useVnetContext();
  const autoFocusRef = useRefAutoFocus<HTMLElement>({
    shouldFocus: visible,
  });

  return (
    // Padding needs to align with the padding of the previous slider step.
    <Box
      p={2}
      ref={mergeRefs([props.refCallback, autoFocusRef])}
      tabIndex={visible ? 0 : -1}
      css={`
        // Do not show the outline when focused. This element cannot be interacted with and we focus
        // it only so that the next tab press is going to focus the VNet header button instead.
        outline: none;
      `}
    >
      <VnetSliderStepHeader goBack={props.prev} />
      <Flex
        p={textSpacing}
        gap={1}
        flexDirection="column"
        css={`
          &:empty {
            display: none;
          }
        `}
      >
        {startAttempt.status === 'error' && (
          <Text>Could not start VNet: {startAttempt.statusText}</Text>
        )}
        {stopAttempt.status === 'error' && (
          <Text>Could not stop VNet: {stopAttempt.statusText}</Text>
        )}

        {status.value === 'stopped' && (
          <Text>VNet automatically authenticates connections to TCP apps.</Text>
        )}
      </Flex>

      {status.value === 'running' && (
        <>
          <Text p={textSpacing}>
            Proxying connections to .teleport-local.dev, .company.private
          </Text>
          <Flex flexDirection="column" gap={1}>
            <AppConnectionItem app="api.company.private" status="on" />
            <AppConnectionItem app="kafka.teleport-local.dev" status="on" />
            <AppConnectionItem
              app="redis.teleport-local.dev"
              status="error"
              error={dnsError}
            />
            <AppConnectionItem
              app="aerospike.teleport-local.dev"
              status="off"
            />
          </Flex>
        </>
      )}
    </Box>
  );
};

const textSpacing = 1;

const dnsError = `DNS query for "redis.teleport-local.dev" in custom DNS zone failed: no matching Teleport app and upstream nameserver did not respond`;
