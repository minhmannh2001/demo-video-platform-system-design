/**
 * WebSocket JSON contract — keep in sync with `internal/ws/protocol.go` (Phần 2).
 * Every envelope uses `v: 1` until a future version bumps the protocol.
 */

export const PROTOCOL_V = 1;

export const CHANNEL_UPLOADS = "uploads" as const;

/** Client → server */
export type ClientMessageType = "subscribe" | "unsubscribe" | "ping";

export type ClientSubscribeByVideo = {
  type: "subscribe";
  v?: number;
  video_id: string;
};

export type ClientSubscribeByChannel = {
  type: "subscribe";
  v?: number;
  channel: typeof CHANNEL_UPLOADS;
};

export type ClientUnsubscribeByVideo = {
  type: "unsubscribe";
  v?: number;
  video_id: string;
};

export type ClientUnsubscribeByChannel = {
  type: "unsubscribe";
  v?: number;
  channel: typeof CHANNEL_UPLOADS;
};

export type ClientPing = { type: "ping"; v?: number };

export type ClientMessage =
  | ClientSubscribeByVideo
  | ClientSubscribeByChannel
  | ClientUnsubscribeByVideo
  | ClientUnsubscribeByChannel
  | ClientPing;

/** Server → client */
export type ServerHello = { type: "hello"; v: number };

export type ServerSubscribed = { type: "subscribed"; v: number; topic: string };

export type ServerUnsubscribed = { type: "unsubscribed"; v: number; topic: string };

export type ServerPong = { type: "pong"; v: number };

export type ServerError = {
  type: "error";
  v: number;
  code: string;
  message?: string;
};

/** Subset aligned with watch API / `models.WatchResponse`. */
export type VideoUpdatedPayload = {
  video_id: string;
  status?: string;
  manifest_url?: string;
  thumbnail_url?: string;
  qualities?: string[];
  renditions?: Array<{
    quality: string;
    width?: number;
    height?: number;
    bitrate?: number;
    playlist_url: string;
  }>;
  message?: string;
};

export type ServerVideoUpdated = {
  type: "video.updated";
  v: number;
  payload: VideoUpdatedPayload;
};

export type ServerCatalogInvalidate = { type: "catalog.invalidate"; v: number };

export type ServerMessage =
  | ServerHello
  | ServerSubscribed
  | ServerUnsubscribed
  | ServerPong
  | ServerError
  | ServerVideoUpdated
  | ServerCatalogInvalidate;
