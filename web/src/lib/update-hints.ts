import type { UpdateCheck } from "./types";

/** 失败分类 → 引导文案 key。仓库未发布正式版时配代理没有意义，所以 404 单独处理。 */
export function updateErrorHintKey(kind?: UpdateCheck["error_kind"]): string {
  switch (kind) {
    case "network":
      return "settings.retryUpdateNetwork";
    case "rate_limited":
      return "settings.updateErrorHintRateLimited";
    case "security":
      return "settings.updateErrorHintSecurity";
    default:
      return "settings.updateErrorHintGeneric";
  }
}
