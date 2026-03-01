import { Namespace, Context } from "@ory/keto-namespace-types"

class profile_user implements Namespace {}

class tenancy_access implements Namespace {
  related: {
    member: (profile_user | tenancy_access)[]
    service: profile_user[]
  }
}

class service_notifications implements Namespace {
  related: {
    owner: profile_user[]
    admin: profile_user[]
    operator: profile_user[]
    viewer: profile_user[]
    member: profile_user[]
    service: (profile_user | tenancy_access)[]

    // Direct permission grants (prefixed with granted_ to avoid
    // name conflicts with OPL permits — Keto skips permit evaluation
    // when a relation shares the same name as a permit function)
    granted_notification_send: (profile_user | service_notifications)[]
    granted_notification_release: (profile_user | service_notifications)[]
    granted_notification_search: (profile_user | service_notifications)[]
    granted_notification_status_view: (profile_user | service_notifications)[]
    granted_notification_status_update: (profile_user | service_notifications)[]
    granted_template_manage: (profile_user | service_notifications)[]
    granted_template_view: (profile_user | service_notifications)[]
  }

  permits = {
    notification_send: (ctx: Context): boolean =>
      this.related.service.includes(ctx.subject) ||
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.operator.includes(ctx.subject) ||
      this.related.granted_notification_send.includes(ctx.subject),

    notification_release: (ctx: Context): boolean =>
      this.related.service.includes(ctx.subject) ||
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.operator.includes(ctx.subject) ||
      this.related.granted_notification_release.includes(ctx.subject),

    notification_search: (ctx: Context): boolean =>
      this.related.service.includes(ctx.subject) ||
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.operator.includes(ctx.subject) ||
      this.related.viewer.includes(ctx.subject) ||
      this.related.member.includes(ctx.subject) ||
      this.related.granted_notification_search.includes(ctx.subject),

    notification_status_view: (ctx: Context): boolean =>
      this.related.service.includes(ctx.subject) ||
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.operator.includes(ctx.subject) ||
      this.related.viewer.includes(ctx.subject) ||
      this.related.member.includes(ctx.subject) ||
      this.related.granted_notification_status_view.includes(ctx.subject),

    notification_status_update: (ctx: Context): boolean =>
      this.related.service.includes(ctx.subject) ||
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.granted_notification_status_update.includes(ctx.subject),

    template_manage: (ctx: Context): boolean =>
      this.related.service.includes(ctx.subject) ||
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.granted_template_manage.includes(ctx.subject),

    template_view: (ctx: Context): boolean =>
      this.related.service.includes(ctx.subject) ||
      this.permits.template_manage(ctx) ||
      this.related.operator.includes(ctx.subject) ||
      this.related.viewer.includes(ctx.subject) ||
      this.related.granted_template_view.includes(ctx.subject),
  }
}
