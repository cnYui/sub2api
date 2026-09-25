export default {
  redeemCards: {
    title: '兑换卡',
    description: '把兑换码做成黑色或白色的 3D 兑换卡，复制链接发给用户',
    editor: {
      title: '制作兑换卡',
      codeLabel: '兑换码',
      codeSearchPlaceholder: '搜索或粘贴兑换码（只列出未使用的）',
      noCodes: '没有找到未使用的兑换码',
      generate: '生成新兑换码',
      change: '换一个',
      existingCard: '这个兑换码已经有卡片，保存会更新卡面，链接不变',
      theme: '颜色',
      themes: {
        dark: '黑色',
        light: '白色'
      },
      fields: {
        amount: '面值',
        plan: '套餐',
        tokens: '用量',
        valid_until: '有效期至',
        serial: '编号',
        heatmap_seed: '热力图'
      },
      serialHint: '卡片正面右下角显示为 No.编号',
      autofill: '按兑换码重新填写',
      reshuffle: '换一张',
      save: '保存并生成链接',
      update: '保存修改',
      saved: '已保存',
      shareLink: '发给用户的链接',
      copyLink: '复制链接',
      openLink: '打开',
      linkCopied: '链接已复制',
      flat: '平面',
      threeD: '3D',
      emptyPreview: '选择或生成一个兑换码后，这里显示卡片'
    },
    list: {
      title: '已制作的兑换卡',
      searchPlaceholder: '搜索兑换码',
      columns: {
        code: '兑换码',
        value: '面值 / 套餐',
        theme: '颜色',
        status: '兑换码状态',
        views: '打开次数',
        createdAt: '创建时间',
        actions: '操作'
      },
      edit: '编辑',
      revoke: '撤销链接',
      revokeConfirm: '撤销后这个链接立即失效，兑换码本身不受影响，之后可以重新制作。确定撤销吗？',
      revoked: '链接已撤销',
      lastViewed: '最近打开：{time}'
    },
    codeStatus: {
      unused: '未使用',
      used: '已兑换',
      expired: '已过期',
      disabled: '已停用'
    },
    profile: {
      button: '卡面信息',
      title: '卡面信息',
      hint: '所有兑换卡共用这些信息，保存后已经发出的卡片也会一起更新',
      ownerLabel: '身份标签',
      ownerName: '名字',
      ownerLines: '简介（每行一条，最多 4 行）',
      steps: '兑换步骤（每行一步，最多 5 步）',
      leftQr: '左侧二维码',
      rightQr: '右侧二维码',
      caption: '二维码下方文字',
      upload: '上传图片',
      useDefault: '用默认图片',
      imageHint: 'PNG / JPG / WebP，不超过 512 KB，建议正方形',
      imageTooLarge: '图片不能超过 512 KB',
      imageInvalid: '只支持 PNG、JPG、WebP 图片',
      resetAll: '全部恢复默认',
      saved: '卡面信息已保存'
    },
    generate: {
      title: '生成新兑换码',
      type: '类型',
      types: {
        balance: '余额',
        balance_package: '余额套餐'
      },
      amount: '金额（USD）',
      plan: '套餐档位',
      planPlaceholder: '选择套餐档位',
      expiry: '兑换码有效期',
      never: '长期有效',
      days: '{days} 天',
      custom: '自定义',
      customDays: '有效天数',
      submit: '生成',
      generated: '已生成兑换码，并选中它来做卡片',
      planRequired: '请选择套餐档位',
      amountRequired: '金额必须大于 0',
      expiryRequired: '有效天数必须是正整数'
    },
    errors: {
      load: '加载兑换卡失败',
      loadCodes: '加载兑换码失败',
      save: '保存失败',
      revoke: '撤销失败',
      loadProfile: '加载卡面信息失败',
      saveProfile: '保存卡面信息失败',
      generate: '生成兑换码失败',
      copy: '复制失败，请手动复制'
    }
  }
}
