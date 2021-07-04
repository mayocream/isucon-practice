CREATE INDEX `price_idx` ON `isuumo`.`chair`(`price`) USING BTREE;
CREATE INDEX `height_idx` ON `isuumo`.`chair`(`height`) USING BTREE;
CREATE INDEX `width_idx` ON `isuumo`.`chair`(`width`) USING BTREE;
CREATE INDEX `depth_idx` ON `isuumo`.`chair`(`depth`) USING BTREE;
CREATE INDEX `kind_idx` ON `isuumo`.`chair`(`kind`) USING BTREE;
CREATE INDEX `color_idx` ON `isuumo`.`chair`(`color`) USING BTREE;
CREATE INDEX `stock_idx` ON `isuumo`.`chair`(`stock`) USING BTREE;

CREATE INDEX `door_height_idx` ON `isuumo`.`estate`(`door_height`) USING BTREE;
CREATE INDEX `door_width_idx` ON `isuumo`.`estate`(`door_width`) USING BTREE;
CREATE INDEX `rent_idx` ON `isuumo`.`estate`(`rent`) USING BTREE;
CREATE INDEX `features_idx` ON `isuumo`.`estate`(`features`) USING BTREE;
CREATE INDEX `popularity_idx` ON `isuumo`.`estate`(`popularity`) USING BTREE;
CREATE INDEX `latitude_idx` ON `isuumo`.`estate`(`latitude`) USING BTREE;
CREATE INDEX `longitude_idx` ON `isuumo`.`estate`(`longitude`) USING BTREE;