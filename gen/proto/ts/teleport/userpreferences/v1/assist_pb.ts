/* eslint-disable */
// @generated by protobuf-ts 2.9.3 with parameter long_type_number,eslint_disable,add_pb_suffix,client_grpc1,server_grpc1,ts_nocheck
// @generated from protobuf file "teleport/userpreferences/v1/assist.proto" (package "teleport.userpreferences.v1", syntax proto3)
// tslint:disable
// @ts-nocheck
//
// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
import type { BinaryWriteOptions } from "@protobuf-ts/runtime";
import type { IBinaryWriter } from "@protobuf-ts/runtime";
import { WireType } from "@protobuf-ts/runtime";
import type { BinaryReadOptions } from "@protobuf-ts/runtime";
import type { IBinaryReader } from "@protobuf-ts/runtime";
import { UnknownFieldHandler } from "@protobuf-ts/runtime";
import type { PartialMessage } from "@protobuf-ts/runtime";
import { reflectionMergePartial } from "@protobuf-ts/runtime";
import { MessageType } from "@protobuf-ts/runtime";
/**
 * AssistUserPreferences is the user preferences for Assist.
 *
 * @generated from protobuf message teleport.userpreferences.v1.AssistUserPreferences
 */
export interface AssistUserPreferences {
    /**
     * preferredLogins is an array of the logins a user would prefer to use when running a command, ordered by preference.
     *
     * @generated from protobuf field: repeated string preferred_logins = 1;
     */
    preferredLogins: string[];
    /**
     * viewMode is the way the assistant is displayed.
     *
     * @generated from protobuf field: teleport.userpreferences.v1.AssistViewMode view_mode = 2;
     */
    viewMode: AssistViewMode;
}
/**
 * AssistViewMode is the way the assistant is displayed.
 *
 * @generated from protobuf enum teleport.userpreferences.v1.AssistViewMode
 */
export enum AssistViewMode {
    /**
     * @generated from protobuf enum value: ASSIST_VIEW_MODE_UNSPECIFIED = 0;
     */
    UNSPECIFIED = 0,
    /**
     * DOCKED is the assistant is docked to the right hand side of the screen.
     *
     * @generated from protobuf enum value: ASSIST_VIEW_MODE_DOCKED = 1;
     */
    DOCKED = 1,
    /**
     * POPUP is the assistant is displayed as a popup.
     *
     * @generated from protobuf enum value: ASSIST_VIEW_MODE_POPUP = 2;
     */
    POPUP = 2,
    /**
     * POPUP_EXPANDED is the assistant is displayed as a popup and expanded.
     *
     * @generated from protobuf enum value: ASSIST_VIEW_MODE_POPUP_EXPANDED = 3;
     */
    POPUP_EXPANDED = 3,
    /**
     * POPUP_EXPANDED_SIDEBAR_VISIBLE is the assistant is displayed as a popup and expanded with the sidebar visible.
     *
     * @generated from protobuf enum value: ASSIST_VIEW_MODE_POPUP_EXPANDED_SIDEBAR_VISIBLE = 4;
     */
    POPUP_EXPANDED_SIDEBAR_VISIBLE = 4
}
// @generated message type with reflection information, may provide speed optimized methods
class AssistUserPreferences$Type extends MessageType<AssistUserPreferences> {
    constructor() {
        super("teleport.userpreferences.v1.AssistUserPreferences", [
            { no: 1, name: "preferred_logins", kind: "scalar", repeat: 2 /*RepeatType.UNPACKED*/, T: 9 /*ScalarType.STRING*/ },
            { no: 2, name: "view_mode", kind: "enum", T: () => ["teleport.userpreferences.v1.AssistViewMode", AssistViewMode, "ASSIST_VIEW_MODE_"] }
        ]);
    }
    create(value?: PartialMessage<AssistUserPreferences>): AssistUserPreferences {
        const message = globalThis.Object.create((this.messagePrototype!));
        message.preferredLogins = [];
        message.viewMode = 0;
        if (value !== undefined)
            reflectionMergePartial<AssistUserPreferences>(this, message, value);
        return message;
    }
    internalBinaryRead(reader: IBinaryReader, length: number, options: BinaryReadOptions, target?: AssistUserPreferences): AssistUserPreferences {
        let message = target ?? this.create(), end = reader.pos + length;
        while (reader.pos < end) {
            let [fieldNo, wireType] = reader.tag();
            switch (fieldNo) {
                case /* repeated string preferred_logins */ 1:
                    message.preferredLogins.push(reader.string());
                    break;
                case /* teleport.userpreferences.v1.AssistViewMode view_mode */ 2:
                    message.viewMode = reader.int32();
                    break;
                default:
                    let u = options.readUnknownField;
                    if (u === "throw")
                        throw new globalThis.Error(`Unknown field ${fieldNo} (wire type ${wireType}) for ${this.typeName}`);
                    let d = reader.skip(wireType);
                    if (u !== false)
                        (u === true ? UnknownFieldHandler.onRead : u)(this.typeName, message, fieldNo, wireType, d);
            }
        }
        return message;
    }
    internalBinaryWrite(message: AssistUserPreferences, writer: IBinaryWriter, options: BinaryWriteOptions): IBinaryWriter {
        /* repeated string preferred_logins = 1; */
        for (let i = 0; i < message.preferredLogins.length; i++)
            writer.tag(1, WireType.LengthDelimited).string(message.preferredLogins[i]);
        /* teleport.userpreferences.v1.AssistViewMode view_mode = 2; */
        if (message.viewMode !== 0)
            writer.tag(2, WireType.Varint).int32(message.viewMode);
        let u = options.writeUnknownFields;
        if (u !== false)
            (u == true ? UnknownFieldHandler.onWrite : u)(this.typeName, message, writer);
        return writer;
    }
}
/**
 * @generated MessageType for protobuf message teleport.userpreferences.v1.AssistUserPreferences
 */
export const AssistUserPreferences = new AssistUserPreferences$Type();
