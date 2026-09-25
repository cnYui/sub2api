export default {
  redeemCards: {
    title: 'Redeem Cards',
    description: 'Turn redeem codes into dark or light 3D cards and send users the link',
    editor: {
      title: 'Make a card',
      codeLabel: 'Redeem code',
      codeSearchPlaceholder: 'Search or paste a code (unused codes only)',
      noCodes: 'No unused redeem codes found',
      generate: 'Generate new code',
      change: 'Change',
      existingCard: 'This code already has a card. Saving updates it and keeps the link.',
      theme: 'Color',
      themes: {
        dark: 'Dark',
        light: 'Light'
      },
      fields: {
        amount: 'Value',
        plan: 'Plan',
        tokens: 'Usage',
        valid_until: 'Valid until',
        serial: 'Serial',
        heatmap_seed: 'Heatmap'
      },
      serialHint: 'Shown as No.<serial> at the bottom right of the front',
      autofill: 'Refill from code',
      reshuffle: 'Shuffle',
      save: 'Save and create link',
      update: 'Save changes',
      saved: 'Saved',
      shareLink: 'Link for the user',
      copyLink: 'Copy link',
      openLink: 'Open',
      linkCopied: 'Link copied',
      flat: 'Flat',
      threeD: '3D',
      emptyPreview: 'Pick or generate a redeem code to preview the card'
    },
    list: {
      title: 'Cards',
      searchPlaceholder: 'Search codes',
      columns: {
        code: 'Code',
        value: 'Value / plan',
        theme: 'Color',
        status: 'Code status',
        views: 'Opens',
        createdAt: 'Created',
        actions: 'Actions'
      },
      edit: 'Edit',
      revoke: 'Revoke link',
      revokeConfirm: 'The link stops working right away. The redeem code itself is not affected and you can make a new card later. Revoke?',
      revoked: 'Link revoked',
      lastViewed: 'Last opened {time}'
    },
    codeStatus: {
      unused: 'Unused',
      used: 'Redeemed',
      expired: 'Expired',
      disabled: 'Disabled'
    },
    profile: {
      button: 'Card details',
      title: 'Card details',
      hint: 'Shared by every card. Saving also updates cards that were already sent.',
      ownerLabel: 'Label',
      ownerName: 'Name',
      ownerLines: 'Bio (one per line, up to 4)',
      steps: 'Redeem steps (one per line, up to 5)',
      leftQr: 'Left QR code',
      rightQr: 'Right QR code',
      caption: 'Caption',
      upload: 'Upload image',
      useDefault: 'Use default',
      imageHint: 'PNG / JPG / WebP, up to 512 KB, square works best',
      imageTooLarge: 'Image must be 512 KB or smaller',
      imageInvalid: 'Only PNG, JPG and WebP images are supported',
      resetAll: 'Reset all to default',
      saved: 'Card details saved'
    },
    generate: {
      title: 'Generate new code',
      type: 'Type',
      types: {
        balance: 'Balance',
        balance_package: 'Balance package'
      },
      amount: 'Amount (USD)',
      plan: 'Package plan',
      planPlaceholder: 'Choose a plan',
      expiry: 'Code expiry',
      never: 'Never expires',
      days: '{days} days',
      custom: 'Custom',
      customDays: 'Days',
      submit: 'Generate',
      generated: 'Code generated and selected for the card',
      planRequired: 'Choose a package plan',
      amountRequired: 'Amount must be greater than 0',
      expiryRequired: 'Days must be a positive whole number'
    },
    errors: {
      load: 'Failed to load cards',
      loadCodes: 'Failed to load redeem codes',
      save: 'Failed to save',
      revoke: 'Failed to revoke',
      loadProfile: 'Failed to load card details',
      saveProfile: 'Failed to save card details',
      generate: 'Failed to generate code',
      copy: 'Copy failed, please copy it manually'
    }
  }
}
