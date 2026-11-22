"use client";

import { useEffect, useState } from "react";

import { useRouter } from "next/navigation";

import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { localizedError } from "@/i18n/errors";
import { api } from "@/lib/api";
import { auth } from "@/lib/auth";

export default function SetupPage() {
  const { t } = useTranslation();
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<unknown>("");
  const [loading, setLoading] = useState(false);
  const [checking, setChecking] = useState(true);
  // 查不到初始化状态时不能默认当成"未初始化"——那样会把初始化表单摆给一个
  // 其实已经设过密码的实例，用户照着填就会覆盖掉原密码。此时关闭入口。
  const [unavailable, setUnavailable] = useState(false);

  useEffect(() => {
    api
      .authStatus()
      .then(({ initialized }) => {
        if (initialized) router.replace("/login");
      })
      .catch((err) => {
        setError(err);
        setUnavailable(true);
      })
      .finally(() => setChecking(false));
  }, [router]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (password !== confirm) {
      setError("auth.passwordMismatch");
      return;
    }
    if (password.length < 8) {
      setError("auth.passwordMin");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const { token } = await api.initPassword(password);
      auth.setToken(token);
      router.replace("/function/tasks");
    } catch (err) {
      setError(err);
    } finally {
      setLoading(false);
    }
  }

  if (checking) return null;

  return (
    <div className="flex min-h-dvh items-center justify-center bg-background p-8">
      <div className="w-full max-w-md space-y-10 py-24 lg:py-32">
        <div className="space-y-4">
          <div className="flex justify-center">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src="/login-mascot.png"
              alt="ARTEX"
              width={220}
              height={211}
              className="select-none"
              draggable={false}
            />
          </div>
          <div className="space-y-4 text-center">
            <h2 className="text-2xl font-medium tracking-tight">
              {unavailable ? t("auth.statusUnknown") : t("auth.initPassword")}
            </h2>
            <p className="mx-auto max-w-xl text-muted-foreground">
              {unavailable ? t("auth.setupUnavailable") : t("auth.setupWelcome")}
            </p>
          </div>
        </div>
        {unavailable ? (
          <div className="flex flex-col gap-4">
            {Boolean(error) && <p className="text-center text-sm text-destructive">{localizedError(error, t)}</p>}
            <Button type="button" className="w-full" onClick={() => window.location.reload()}>
              {t("common.retry")}
            </Button>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div className="space-y-1.5">
              <Label htmlFor="password">{t("account.newPassword")}</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={t("auth.minPlaceholder")}
                autoFocus
                autoComplete="new-password"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="confirm">{t("auth.confirmPassword")}</Label>
              <Input
                id="confirm"
                type="password"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                placeholder={t("auth.reenterPassword")}
                autoComplete="new-password"
              />
            </div>
            {Boolean(error) && <p className="text-sm text-destructive">{localizedError(error, t)}</p>}
            <Button type="submit" className="w-full" disabled={loading || !password || !confirm}>
              {loading ? t("auth.saving") : t("auth.setupAndLogin")}
            </Button>
          </form>
        )}
      </div>
    </div>
  );
}
